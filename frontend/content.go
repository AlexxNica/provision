package frontend

import (
	"fmt"
	"net/http"

	"github.com/digitalrebar/provision/midlayer"
	"github.com/digitalrebar/provision/models"
	"github.com/digitalrebar/store"
	"github.com/gin-gonic/gin"
)

type ContentMetaData struct {
	// required: true
	Name        string
	Source      string
	Description string
	Version     string
}

//
// Isos???
// Files??
//
// swagger:model
type Content struct {
	// required: true
	Meta ContentMetaData `json:"meta"`

	/*
		        These are the sections:

			tasks        map[string]*models.Task
			bootenvs     map[string]*models.BootEnv
			templates    map[string]*models.Template
			profiles     map[string]*models.Profile
			params       map[string]*models.Param
			reservations map[string]*models.Reservation
			subnets      map[string]*models.Subnet
			users        map[string]*models.User
			preferences  map[string]*models.Pref
			plugins      map[string]*models.Plugin
			machines     map[string]*models.Machine
			leases       map[string]*models.Lease
	*/
	Sections Sections `json:"sections"`
}

type Section map[string]interface{}
type Sections map[string]Section

// swagger:model
type ContentSummary struct {
	Meta   ContentMetaData `json:"meta"`
	Counts map[string]int
}

// ContentsResponse returned on a successful GET of a contents
// swagger:response
type ContentResponse struct {
	// in: body
	Body *Content
}

// ContentSummaryResponse returned on a successful Post of a content
// swagger:response
type ContentSummaryResponse struct {
	// in: body
	Body *ContentSummary
}

// ContentsResponse returned on a successful GET of all contents
// swagger:response
type ContentsResponse struct {
	// in: body
	Body []*ContentSummary
}

// swagger:parameters uploadContent createContent
type ContentBodyParameter struct {
	// in: body
	Body *Content
}

// swagger:parameters getContent deleteContent uploadContent
type ContentParameter struct {
	// in: path
	Name string `json:"name"`
}

func (f *Frontend) buildNewStore(content *Content) (newStore store.Store, err error) {
	filename := fmt.Sprintf("file:///%s/%s-%s.yaml?codec=yaml", f.SaasDir, content.Meta.Name, content.Meta.Version)

	newStore, err = store.Open(filename)
	if err != nil {
		return
	}

	if md, ok := newStore.(store.MetaSaver); ok {
		data := map[string]string{
			"Name":        content.Meta.Name,
			"Source":      content.Meta.Source,
			"Description": content.Meta.Description,
			"Version":     content.Meta.Version,
		}
		md.SetMetaData(data)
	}

	for prefix, objs := range content.Sections {
		var sub store.Store
		sub, err = newStore.MakeSub(prefix)
		if err != nil {
			return
		}

		for k, obj := range objs {
			err = sub.Save(k, obj)
			if err != nil {
				return
			}
		}
	}

	return
}

func buildSummary(st store.Store) *ContentSummary {
	mst, ok := st.(store.MetaSaver)
	if !ok {
		return nil
	}
	cs := &ContentSummary{}
	metaData := mst.MetaData()

	cs.Meta.Name = metaData["Name"]
	cs.Meta.Source = metaData["Source"]
	cs.Meta.Description = metaData["Description"]
	cs.Meta.Version = metaData["Version"]
	cs.Counts = map[string]int{}

	subs := mst.Subs()
	for k, sub := range subs {
		keys, err := sub.Keys()
		if err != nil {
			continue
		}
		cs.Counts[k] = len(keys)
	}

	return cs
}

func (f *Frontend) buildContent(st store.Store) (*Content, *models.Error) {
	content := &Content{}

	var md map[string]string
	mst, ok := st.(store.MetaSaver)
	if ok {
		md = mst.MetaData()
	} else {
		md = map[string]string{}
	}

	// Copy in MetaData
	if val, ok := md["Name"]; ok {
		content.Meta.Name = val
	} else {
		content.Meta.Name = "Unknown"
	}
	if val, ok := md["Source"]; ok {
		content.Meta.Source = val
	} else {
		content.Meta.Source = "Unknown"
	}
	if val, ok := md["Description"]; ok {
		content.Meta.Description = val
	} else {
		content.Meta.Description = "Unknown"
	}
	if val, ok := md["Version"]; ok {
		content.Meta.Version = val
	} else {
		content.Meta.Version = "Unknown"
	}

	// Walk subs to build content sets
	content.Sections = Sections{}
	for prefix, sub := range st.Subs() {
		_, err := models.New(prefix)
		if err != nil {
			berr := models.NewError("ValidationError", http.StatusUnprocessableEntity, err.Error())
			return nil, berr
		}

		keys, err := sub.Keys()
		if err != nil {
			berr := models.NewError("ServerError", http.StatusInternalServerError, err.Error())
			return nil, berr
		}
		objs := make(Section, 0)
		for _, k := range keys {
			// This is protected by the earlier check
			v, _ := models.New(prefix)
			err := sub.Load(k, &v)
			if err != nil {
				berr := models.NewError("ServerError", http.StatusInternalServerError, err.Error())
				return nil, berr
			}
			objs[k] = v
		}

		content.Sections[prefix] = objs
	}

	return content, nil
}

func (f *Frontend) findContent(name string) (cst store.Store) {
	if stack, ok := f.dt.Backend.(*midlayer.DataStack); !ok {
		mst, ok := f.dt.Backend.(store.MetaSaver)
		if !ok {
			return nil
		}
		metaData := mst.MetaData()
		if metaData["Name"] == name {
			cst = f.dt.Backend
		}
	} else {
		for _, st := range stack.Layers() {
			mst, ok := st.(store.MetaSaver)
			if !ok {
				continue
			}
			metaData := mst.MetaData()
			if metaData["Name"] == name {
				cst = st
				break
			}
		}
	}
	return
}

func (f *Frontend) InitContentApi() {
	// swagger:route GET /contents Contents listContents
	//
	// Lists possible contents on the system to serve DHCP
	//
	//     Produces:
	//       application/json
	//
	//     Responses:
	//       200: ContentsResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       500: ErrorResponse
	f.ApiGroup.GET("/contents",
		func(c *gin.Context) {
			if !assureAuth(c, f.Logger, "contents", "list", "") {
				return
			}

			contents := []*ContentSummary{}
			func() {
				f.dt.Stop()
				defer f.dt.Start()

				if stack, ok := f.dt.Backend.(*midlayer.DataStack); !ok {
					cs := buildSummary(f.dt.Backend)
					if cs != nil {
						contents = append(contents, cs)
					}
				} else {
					for _, st := range stack.Layers() {
						cs := buildSummary(st)
						if cs != nil {
							contents = append(contents, cs)
						}
					}
				}
			}()

			c.JSON(http.StatusOK, contents)
		})

	// swagger:route GET /contents/{name} Contents getContent
	//
	// Get a specific content with {name}
	//
	// Get a specific content specified by {name}.
	//
	//     Produces:
	//       application/json
	//
	//     Responses:
	//       200: ContentResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	//       500: ErrorResponse
	f.ApiGroup.GET("/contents/:name",
		func(c *gin.Context) {
			name := c.Param(`name`)
			if !assureAuth(c, f.Logger, "contents", "get", name) {
				return
			}

			func() {
				f.dt.Stop()
				defer f.dt.Start()

				if cst := f.findContent(name); cst == nil {
					c.JSON(http.StatusNotFound,
						models.NewError("API_ERROR", http.StatusNotFound,
							fmt.Sprintf("content get: not found: %s", name)))
				} else {
					content, err := f.buildContent(cst)
					if err != nil {
						c.JSON(err.Code, err)
					} else {
						c.JSON(http.StatusOK, content)
					}
				}
			}()
		})

	// swagger:route POST /contents Contents createContent
	//
	// Create content into Digital Rebar Provision
	//
	//     Responses:
	//       201: ContentSummaryResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       403: ErrorResponse
	//       404: ErrorResponse
	//       409: ErrorResponse
	//       415: ErrorResponse
	//       422: ErrorResponse
	//       500: ErrorResponse
	//       507: ErrorResponse
	f.ApiGroup.POST("/contents",
		func(c *gin.Context) {
			if !assureAuth(c, f.Logger, "contents", "create", "*") {
				return
			}
			content := &Content{}
			if !assureDecode(c, content) {
				return
			}

			name := content.Meta.Name
			func() {
				f.dt.Stop()
				defer f.dt.Start()

				if cst := f.findContent(name); cst != nil {
					c.JSON(http.StatusConflict,
						models.NewError("API_ERROR", http.StatusConflict,
							fmt.Sprintf("content post: already exists: %s", name)))
					return
				}

				if newStore, err := f.buildNewStore(content); err != nil {
					jsonError(c, err, http.StatusInternalServerError,
						fmt.Sprintf("failed to build content: %s: ", name))
					return
				} else {
					cs := buildSummary(newStore)

					ds := f.dt.Backend.(*midlayer.DataStack)
					if nbs, err := ds.AddReplaceStore(name, newStore, f.Logger); err != nil {
						midlayer.CleanUpStore(newStore)
						jsonError(c, err, http.StatusInternalServerError,
							fmt.Sprintf("failed to add content: %s: ", name))
					} else {
						f.dt.ReplaceBackend(nbs)
						c.JSON(http.StatusCreated, cs)
					}
				}
			}()
		})

	// swagger:route PUT /contents/{name} Contents uploadContent
	//
	// Replace content in Digital Rebar Provision
	//
	//     Responses:
	//       200: ContentSummaryResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       403: ErrorResponse
	//       404: ErrorResponse
	//       409: ErrorResponse
	//       415: ErrorResponse
	//       422: ErrorResponse
	//       500: ErrorResponse
	//       507: ErrorResponse
	f.ApiGroup.PUT("/contents/:name",
		func(c *gin.Context) {
			if !assureAuth(c, f.Logger, "contents", "update", "*") {
				return
			}
			content := &Content{}
			if !assureDecode(c, content) {
				return
			}

			name := c.Param(`name`)
			if name != content.Meta.Name {
				c.JSON(http.StatusBadRequest,
					models.NewError("API_ERROR", http.StatusBadRequest,
						fmt.Sprintf("Name must match: %s != %s\n", content.Meta.Name, c.Param(`name`))))
				return

			}

			func() {
				f.dt.Stop()
				defer f.dt.Start()

				if cst := f.findContent(name); cst == nil {
					c.JSON(http.StatusNotFound,
						models.NewError("API_ERROR", http.StatusNotFound,
							fmt.Sprintf("content put: not found: %s", name)))
					return
				}

				if newStore, err := f.buildNewStore(content); err != nil {
					jsonError(c, err, http.StatusInternalServerError,
						fmt.Sprintf("failed to build content: %s: ", name))
					return
				} else {
					cs := buildSummary(newStore)

					ds := f.dt.Backend.(*midlayer.DataStack)
					if nbs, err := ds.AddReplaceStore(name, newStore, f.Logger); err != nil {
						midlayer.CleanUpStore(newStore)
						jsonError(c, err, http.StatusInternalServerError,
							fmt.Sprintf("failed to replace content: %s: ", name))
					} else {
						f.dt.ReplaceBackend(nbs)
						c.JSON(http.StatusOK, cs)
					}
				}
			}()
		})

	// swagger:route DELETE /contents/{name} Contents deleteContent
	//
	// Delete a content set.
	//
	//     Responses:
	//       204: NoContentResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.DELETE("/contents/:name",
		func(c *gin.Context) {
			name := c.Param(`name`)
			if !assureAuth(c, f.Logger, "contents", "delete", name) {
				return
			}

			func() {
				f.dt.Stop()
				defer f.dt.Start()

				cst := f.findContent(name)
				if cst == nil {
					c.JSON(http.StatusNotFound,
						models.NewError("API_ERROR", http.StatusNotFound,
							fmt.Sprintf("content get: not found: %s", name)))
					return
				}

				ds := f.dt.Backend.(*midlayer.DataStack)
				if nbs, err := ds.RemoveStore(name, f.Logger); err != nil {
					jsonError(c, err, http.StatusInternalServerError,
						fmt.Sprintf("failed to remove content: %s: ", name))
				} else {
					f.dt.ReplaceBackend(nbs)
					c.Data(http.StatusNoContent, gin.MIMEJSON, nil)
				}

			}()
		})
}
