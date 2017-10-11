package midlayer

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/digitalrebar/provision/backend"
	"github.com/digitalrebar/provision/models"
	"github.com/digitalrebar/store"
)

type DataStack struct {
	store.StackedStore

	writeContent   store.Store
	localContent   store.Store
	saasContents   map[string]store.Store
	defaultContent store.Store
	pluginContents map[string]store.Store
	basicContent   store.Store
}

func CleanUpStore(st store.Store) error {
	st.Close()
	switch st.Type() {
	case "bolt":
		fst, _ := st.(*store.Bolt)
		return os.Remove(fst.Path)
	case "file":
		fst, _ := st.(*store.File)
		return os.Remove(fst.Path)
	case "directory":
		fst, _ := st.(*store.Directory)
		return os.RemoveAll(fst.Path)
	default:
		return nil
	}
}

func (d *DataStack) Clone() *DataStack {
	dtStore := &DataStack{
		StackedStore:   store.StackedStore{},
		writeContent:   d.writeContent,
		localContent:   d.localContent,
		basicContent:   d.basicContent,
		defaultContent: d.defaultContent,
		saasContents:   map[string]store.Store{},
		pluginContents: map[string]store.Store{},
	}
	dtStore.Open(store.DefaultCodec)
	for k, s := range d.saasContents {
		dtStore.saasContents[k] = s
	}
	for k, s := range d.pluginContents {
		dtStore.pluginContents[k] = s
	}

	return dtStore
}

func (d *DataStack) rebuild(oldStore store.Store, logger *log.Logger) (*DataStack, error, error) {
	if err := d.buildStack(); err != nil {
		return nil, models.NewError("ValidationError", 422, err.Error()), nil
	}
	hard, soft := backend.ValidateDataTrackerStore(d, logger)
	if hard == nil && oldStore != nil {
		CleanUpStore(oldStore)
	}
	return d, hard, soft
}

func (d *DataStack) RemoveSAAS(name string, logger *log.Logger) (*DataStack, error, error) {
	dtStore := d.Clone()
	oldStore, _ := dtStore.saasContents[name]
	delete(dtStore.saasContents, name)
	return dtStore.rebuild(oldStore, logger)
}

func (d *DataStack) AddReplaceSAAS(name string, newStore store.Store, logger *log.Logger) (*DataStack, error, error) {
	dtStore := d.Clone()
	oldStore, _ := dtStore.saasContents[name]
	dtStore.saasContents[name] = newStore
	return dtStore.rebuild(oldStore, logger)
}

func (d *DataStack) RemovePlugin(name string, logger *log.Logger) (*DataStack, error, error) {
	dtStore := d.Clone()
	oldStore, _ := dtStore.pluginContents[name]
	delete(dtStore.pluginContents, name)
	return dtStore.rebuild(oldStore, logger)
}

func (d *DataStack) AddReplacePlugin(name string, newStore store.Store, logger *log.Logger) (*DataStack, error, error) {
	dtStore := d.Clone()
	oldStore, _ := dtStore.pluginContents[name]
	dtStore.pluginContents[name] = newStore
	return dtStore.rebuild(oldStore, logger)
}

func (d *DataStack) buildStack() error {
	if err := d.Push(d.writeContent, false, true); err != nil {
		return err
	}
	if d.localContent != nil {
		if err := d.Push(d.localContent, false, false); err != nil {
			return err
		}
	}

	// Sort Names
	saas := make([]string, 0, len(d.saasContents))
	for k, _ := range d.saasContents {
		saas = append(saas, k)
	}
	sort.Strings(saas)

	for _, k := range saas {
		if err := d.Push(d.saasContents[k], true, false); err != nil {
			return err
		}
	}

	if d.defaultContent != nil {
		if err := d.Push(d.defaultContent, false, false); err != nil {
			return err
		}
	}

	plugins := make([]string, 0, len(d.pluginContents))
	for k, _ := range d.pluginContents {
		plugins = append(plugins, k)
	}
	sort.Strings(plugins)

	for _, k := range plugins {
		if err := d.Push(d.pluginContents[k], true, false); err != nil {
			return err
		}
	}
	d.Push(d.basicContent, false, false)
	return nil
}

func DefaultDataStack(dataRoot, backendType, localContent, defaultContent, saasDir string) (*DataStack, error) {
	dtStore := &DataStack{
		StackedStore:   store.StackedStore{},
		saasContents:   map[string]store.Store{},
		pluginContents: map[string]store.Store{},
	}
	dtStore.Open(store.DefaultCodec)
	dtStore.basicContent = backend.BasicContent()

	var backendStore store.Store
	if u, err := url.Parse(backendType); err == nil && u.Scheme != "" {
		backendStore, err = store.Open(backendType)
		if err != nil {
			return nil, fmt.Errorf("Failed to open backend content %v: %v", backendType, err)
		}
	} else {
		storeURI := fmt.Sprintf("%s://%s", backendType, dataRoot)
		backendStore, err = store.Open(storeURI)
		if err != nil {
			return nil, fmt.Errorf("Failed to open backend content (%s): %v", storeURI, err)
		}
	}
	if md, ok := backendStore.(store.MetaSaver); ok {
		data := map[string]string{"Name": "BackingStore", "Description": "Writable backing store", "Version": "user"}
		md.SetMetaData(data)
	}
	dtStore.writeContent = backendStore

	if localContent != "" {
		etcStore, err := store.Open(localContent)
		if err != nil {
			return nil, fmt.Errorf("Failed to open local content: %v", err)
		}
		dtStore.localContent = etcStore
		if md, ok := etcStore.(store.MetaSaver); ok {
			d := md.MetaData()
			if _, ok := d["Name"]; !ok {
				data := map[string]string{"Name": "LocalStore", "Description": "Local Override Store", "Version": "user"}
				md.SetMetaData(data)
			}
		}
	}

	// Add SAAS content stores to the DataTracker store here
	dtStore.saasContents = make(map[string]store.Store)
	err := filepath.Walk(saasDir, func(filepath string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			ext := path.Ext(filepath)
			codec := "json"
			if ext == ".yaml" || ext == ".yml" {
				codec = "yaml"
			}

			fs, err := store.Open(fmt.Sprintf("file://%s?codec=%s", filepath, codec))
			if err != nil {
				return err
			}

			mst, _ := fs.(store.MetaSaver)
			md := mst.MetaData()
			name := md["Name"]

			dtStore.saasContents[name] = fs
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to open backend content: %v", err)
	}

	if defaultContent != "" {
		defaultStore, err := store.Open(defaultContent)
		if err != nil {
			return nil, fmt.Errorf("Failed to open default content: %v", err)
		}
		dtStore.defaultContent = defaultStore
		if md, ok := defaultStore.(store.MetaSaver); ok {
			d := md.MetaData()
			if _, ok := d["Name"]; !ok {
				data := map[string]string{"Name": "DefaultStore", "Description": "Initial Default Content", "Version": "user"}
				md.SetMetaData(data)
			}
		}
	}
	return dtStore, dtStore.buildStack()
}
