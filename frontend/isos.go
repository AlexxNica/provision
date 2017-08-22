package frontend

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/digitalrebar/provision/backend"
	"github.com/digitalrebar/provision/models"
	"github.com/gin-gonic/gin"
)

type IsoPaths []string

// IsosResponse returned on a successful GET of isos
// swagger:response
type IsosResponse struct {
	// in: body
	Body IsoPaths
}

// XXX: One day resolve the binary blob appropriately:
// {
//   "name": "BinaryData",
//   "in": "body",
//   "required": true,
//   "schema": {
//     "type": "string",
//     "format": "byte"
//   }
// }
//

// IsoResponse returned on a successful GET of an iso
// swagger:response
type IsoResponse struct {
	// in: body
	Body interface{}
}

// swagger:model
type IsoInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// IsoInfoResponse returned on a successful upload of an iso
// swagger:response
type IsoInfoResponse struct {
	// in: body
	Body *IsoInfo
}

// swagger:parameters uploadIso getIso deleteIso
type IsoPathPathParameter struct {
	// in: path
	Path string `json:"path"`
}

// IsoData body of the upload
// swagger:parameters uploadIso
type IsoData struct {
	// in: body
	Body interface{}
}

func (f *Frontend) InitIsoApi() {
	// swagger:route GET /isos Isos listIsos
	//
	// Lists isos in isos directory
	//
	// Lists the isos in a directory under /isos.
	//
	//     Responses:
	//       200: IsosResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	f.ApiGroup.GET("/isos",
		func(c *gin.Context) {
			if !assureAuth(c, f.Logger, "isos", "list", "") {
				return
			}
			ents, err := ioutil.ReadDir(path.Join(f.FileRoot, "isos"))
			if err != nil {
				c.JSON(http.StatusNotFound,
					models.NewError("API ERROR", http.StatusNotFound, fmt.Sprintf("list: error listing isos: %v", err)))
				return
			}
			res := []string{}
			for _, ent := range ents {
				if !ent.Mode().IsRegular() {
					continue
				}
				res = append(res, ent.Name())
			}
			c.JSON(http.StatusOK, res)
		})
	// swagger:route GET /isos/{path} Isos getIso
	//
	// Get a specific Iso with {path}
	//
	// Get a specific iso specified by {path} under isos.
	//
	//     Produces:
	//       application/octet-stream
	//       application/json
	//
	//     Responses:
	//       200: IsoResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	f.ApiGroup.GET("/isos/:name",
		func(c *gin.Context) {
			if !assureAuth(c, f.Logger, "isos", "get", c.Param(`name`)) {
				return
			}
			isoName := path.Join(f.FileRoot, `isos`, path.Base(c.Param(`name`)))
			c.File(isoName)
		})
	// swagger:route POST /isos/{path} Isos uploadIso
	//
	// Upload an iso to a specific {path} in the tree under isos.
	//
	// The iso will be uploaded to the {path} in /isos.  The {path} will be created.
	//
	//     Consumes:
	//       application/octet-stream
	//
	//     Produces:
	//       application/json
	//
	//     Responses:
	//       201: IsoInfoResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	//       409: ErrorResponse
	//       415: ErrorResponse
	//       507: ErrorResponse
	f.ApiGroup.POST("/isos/:name",
		func(c *gin.Context) {
			if !assureAuth(c, f.Logger, "isos", "post", c.Param(`name`)) {
				return
			}
			uploadIso(c, f.FileRoot, c.Param(`name`), f.dt)
		})
	// swagger:route DELETE /isos/{path} Isos deleteIso
	//
	// Delete an iso to a specific {path} in the tree under isos.
	//
	// The iso will be removed from the {path} in /isos.
	//
	//     Responses:
	//       204: NoContentResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	f.ApiGroup.DELETE("/isos/:name",
		func(c *gin.Context) {
			name := c.Param(`name`)
			if !assureAuth(c, f.Logger, "isos", "delete", name) {
				return
			}
			isoName := path.Join(f.FileRoot, `isos`, path.Base(name))
			if err := os.Remove(isoName); err != nil {
				c.JSON(http.StatusNotFound,
					models.NewError("API ERROR", http.StatusNotFound, fmt.Sprintf("delete: unable to delete %s", name)))
				return
			}
			c.Data(http.StatusNoContent, gin.MIMEJSON, nil)
		})
}

func reloadBootenvsForIso(dt *backend.DataTracker, name string) {
	ref := &backend.BootEnv{}
	d, unloader := dt.LockEnts(ref.Locks("update")...)
	defer unloader()

	for _, blob := range d("bootenvs").Items() {
		env := backend.AsBootEnv(blob)
		if env.Available || env.OS.IsoFile != name {
			continue
		}
		env.Available = true
		dt.Update(d, env)
	}
}

func uploadIso(c *gin.Context, fileRoot, name string, dt *backend.DataTracker) {
	if err := os.MkdirAll(path.Join(fileRoot, `isos`), 0755); err != nil {
		c.JSON(http.StatusConflict,
			models.NewError("API_ERROR", http.StatusConflict, fmt.Sprintf("upload: unable to create isos directory")))
		return
	}
	var copied int64
	ctype := c.Request.Header.Get(`Content-Type`)
    switch strings.Split(ctype, "; ")[0] {
    case `application/octet-stream`:
		if c.Request.Body == nil {
			c.JSON(http.StatusBadRequest,
				models.NewError("API ERROR", http.StatusBadRequest,
					fmt.Sprintf("upload: Unable to upload %s: missing body", name)))
			return
		}
		isoTmpName := path.Join(fileRoot, `isos`, fmt.Sprintf(`.%s.part`, path.Base(name)))
		isoName := path.Join(fileRoot, `isos`, path.Base(name))
		if _, err := os.Open(isoTmpName); err == nil {
			c.JSON(http.StatusConflict,
				models.NewError("API ERROR", http.StatusConflict, fmt.Sprintf("upload: iso %s already uploading", name)))
			return
		}
		tgt, err := os.Create(isoTmpName)

		if err != nil {
			c.JSON(http.StatusConflict,
				models.NewError("API ERROR", http.StatusConflict, fmt.Sprintf("upload: Unable to upload %s: %v", name, err)))
			return
		}

		copied, err = io.Copy(tgt, c.Request.Body)
		tgt.Close()
		if err != nil {
			os.Remove(isoTmpName)
			c.JSON(http.StatusInsufficientStorage,
				models.NewError("API ERROR",
					http.StatusInsufficientStorage, fmt.Sprintf("upload: Failed to upload %s: %v", name, err)))
			return
		}
		if c.Request.ContentLength > 0 && copied != c.Request.ContentLength {
			os.Remove(isoTmpName)
			c.JSON(http.StatusBadRequest,
				models.NewError("API ERROR", http.StatusBadRequest,
					fmt.Sprintf("upload: Failed to upload entire file %s: %d bytes expected, %d bytes received", name, c.Request.ContentLength, copied)))
			return
		}
		os.Remove(isoName)
		os.Rename(isoTmpName, isoName)

    case `multipart/form-data`:
        header , err := c.FormFile("file")
        if err != nil {
            c.JSON(http.StatusBadRequest,
				models.NewError("API ERROR", http.StatusBadRequest,
					fmt.Sprintf("upload: Failed to find file upload header: %v: %d bytes expected, %d bytes received", err)))
			return
		}
		isoTmpName := path.Join(fileRoot, `isos`, fmt.Sprintf(`.%s.part`, path.Base(header.Filename)))
		isoName := path.Join(fileRoot, `isos`, path.Base(header.Filename))
        out, err := os.Create(isoTmpName)
        if err != nil {
			c.JSON(http.StatusConflict,
				models.NewError("API ERROR", http.StatusConflict, fmt.Sprintf("upload: iso %s already uploading", header.Filename)))
			return
        }
        defer out.Close()
        file, err := header.Open()
        if err != nil {
			c.JSON(http.StatusConflict,
				models.NewError("API ERROR", http.StatusBadRequest, fmt.Sprintf("upload: iso %s invalid form data", header.Filename)))
			return
        }
        defer file.Close()
        copied, err = io.Copy(out, file)
        if err != nil {
			c.JSON(http.StatusConflict,
				models.NewError("API ERROR", http.StatusBadRequest, fmt.Sprintf("upload: iso %s could not save", header.Filename)))
			return
        }
		os.Remove(isoName)
		os.Rename(isoTmpName, isoName)
    default:
		c.JSON(http.StatusUnsupportedMediaType,
			models.NewError("API ERROR", http.StatusUnsupportedMediaType,
				fmt.Sprintf("upload: iso %s content-type %s is not application/octet-stream or multipart/form-data", name, ctype)))
		return
	}
	go reloadBootenvsForIso(dt, name)
	c.JSON(http.StatusCreated, &IsoInfo{Path: name, Size: copied})
}
