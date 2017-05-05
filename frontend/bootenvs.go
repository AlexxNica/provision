package frontend

import (
	"github.com/VictorLowther/jsonpatch2"
	"github.com/digitalrebar/provision/backend"
	"github.com/gin-gonic/gin"
)

// BootEnvResponse returned on a successful GET, PUT, PATCH, or POST of a single bootenv
// swagger:response
type BootEnvResponse struct {
	// in: body
	Body *backend.BootEnv
}

// BootEnvsResponse returned on a successful GET of all the bootenvs
// swagger:response
type BootEnvsResponse struct {
	//in: body
	Body []*backend.BootEnv
}

// BootEnvBodyParameter used to inject a BootEnv
// swagger:parameters createBootEnv putBootEnv
type BootEnvBodyParameter struct {
	// in: body
	// required: true
	Body *backend.BootEnv
}

// BootEnvPatchBodyParameter used to patch a BootEnv
// swagger:parameters patchBootEnv
type BootEnvPatchBodyParameter struct {
	// in: body
	// required: true
	Body jsonpatch2.Patch
}

// BootEnvPathParameter used to name a BootEnv in the path
// swagger:parameters putBootEnvs getBootEnv putBootEnv patchBootEnv deleteBootEnv
type BootEnvPathParameter struct {
	// in: path
	// required: true
	Name string `json:"name"`
}

// BootEnvListPathParameter used to limit lists of BootEnv by path options
// swagger:parameters listBootEnvs
type BootEnvListPathParameter struct {
	// in: query
	Offest int `json:"offset"`
	// in: query
	Limit int `json:"limit"`
	// in: query
	Available bool
	// in: query
	OnlyUnknown bool
	// in: Name
	Name string
}

func (f *Frontend) InitBootEnvApi() {
	// swagger:route GET /bootenvs BootEnvs listBootEnvs
	//
	// Lists BootEnvs filtered by some parameters.
	//
	// This will show all BootEnvs by default.
	//
	//     Responses:
	//       200: BootEnvsResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	f.ApiGroup.GET("/bootenvs",
		func(c *gin.Context) {
			f.List(c, f.dt.NewBootEnv())
		})

	// swagger:route POST /bootenvs BootEnvs createBootEnv
	//
	// Create a BootEnv
	//
	// Create a BootEnv from the provided object
	//
	//     Responses:
	//       201: BootEnvResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       422: ErrorResponse
	f.ApiGroup.POST("/bootenvs",
		func(c *gin.Context) {
			b := f.dt.NewBootEnv()
			f.Create(c, b)
		})
	// swagger:route GET /bootenvs/{name} BootEnvs getBootEnv
	//
	// Get a BootEnv
	//
	// Get the BootEnv specified by {name} or return NotFound.
	//
	//     Responses:
	//       200: BootEnvResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	f.ApiGroup.GET("/bootenvs/:name",
		func(c *gin.Context) {
			f.Fetch(c, f.dt.NewBootEnv(), c.Param(`name`))
		})

	// swagger:route PATCH /bootenvs/{name} BootEnvs patchBootEnv
	//
	// Patch a BootEnv
	//
	// Update a BootEnv specified by {name} using a RFC6902 Patch structure
	//
	//     Responses:
	//       200: BootEnvResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	//       406: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.PATCH("/bootenvs/:name",
		func(c *gin.Context) {
			f.Patch(c, f.dt.NewBootEnv(), c.Param(`name`))
		})

	// swagger:route PUT /bootenvs/{name} BootEnvs putBootEnv
	//
	// Put a BootEnv
	//
	// Update a BootEnv specified by {name} using a JSON BootEnv
	//
	//     Responses:
	//       200: BootEnvResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.PUT("/bootenvs/:name",
		func(c *gin.Context) {
			f.Update(c, f.dt.NewBootEnv(), c.Param(`name`))
		})

	// swagger:route DELETE /bootenvs/{name} BootEnvs deleteBootEnv
	//
	// Delete a BootEnv
	//
	// Delete a BootEnv specified by {name}
	//
	//     Responses:
	//       200: BootEnvResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	f.ApiGroup.DELETE("/bootenvs/:name",
		func(c *gin.Context) {
			b := f.dt.NewBootEnv()
			b.Name = c.Param(`name`)
			f.Remove(c, b)

		})
}
