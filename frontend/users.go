package frontend

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rackn/rocket-skates/backend"
)

// UserResponse returned on a successful GET, PUT, PATCH, or POST of a single user
// swagger:response
type UserResponse struct {
	// in: body
	Body *backend.User
}

// UsersResponse returned on a successful GET of all the users
// swagger:response
type UsersResponse struct {
	//in: body
	Body []*backend.User
}

// UserBodyParameter used to inject a User
// swagger:parameters createUser putUser
type UserBodyParameter struct {
	// in: body
	// required: true
	Body *backend.User
}

// UserPatchBodyParameter used to patch a User
// swagger:parameters patchUser
type UserPatchBodyParameter struct {
	// in: body
	// required: true
	Body []JSONPatchOperation
}

// UserPathParameter used to name a User in the path
// swagger:parameters getUser putUser patchUser deleteUser
type UserPathParameter struct {
	// in: path
	// required: true
	Name string `json:"name"`
}

func (f *Frontend) InitUserApi() {
	// swagger:route GET /users Users listUsers
	//
	// Lists Users filtered by some parameters.
	//
	// This will show all Users by default.
	//
	//     Responses:
	//       200: UsersResponse
	//       401: ErrorResponse
	f.ApiGroup.GET("/users",
		func(c *gin.Context) {
			f.List(c, f.dt.NewUser())
		})

	// swagger:route POST /users Users createUser
	//
	// Create a User
	//
	// Create a User from the provided object
	//
	//     Responses:
	//       201: UserResponse
	//       400: ErrorResponse
	//       401: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.POST("/users",
		func(c *gin.Context) {
			b := f.dt.NewUser()
			f.Create(c, b)
		})

	// swagger:route GET /users/{name} Users getUser
	//
	// Get a User
	//
	// Get the User specified by {name} or return NotFound.
	//
	//     Responses:
	//       200: UserResponse
	//       401: ErrorResponse
	//       404: ErrorResponse
	f.ApiGroup.GET("/users/:name",
		func(c *gin.Context) {
			f.Fetch(c, f.dt.NewUser(), c.Param(`name`))
		})

	// swagger:route PATCH /users/{name} Users patchUser
	//
	// Patch a User
	//
	// Update a User specified by {name} using a RFC6902 Patch structure
	//
	//     Responses:
	//       200: UserResponse
	//       400: ErrorResponse
	//       401: ErrorResponse
	//       404: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.PATCH("/users/:name",
		func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, backend.NewError("API_ERROR", http.StatusNotImplemented, "user patch: NOT IMPLEMENTED"))
		})

	// swagger:route PUT /users/{name} Users putUser
	//
	// Put a User
	//
	// Update a User specified by {name} using a JSON User
	//
	//     Responses:
	//       200: UserResponse
	//       400: ErrorResponse
	//       401: ErrorResponse
	//       404: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.PUT("/users/:name",
		func(c *gin.Context) {
			f.Update(c, f.dt.NewUser(), c.Param(`name`))
		})

	// swagger:route DELETE /users/{name} Users deleteUser
	//
	// Delete a User
	//
	// Delete a User specified by {name}
	//
	//     Responses:
	//       200: UserResponse
	//       401: ErrorResponse
	//       404: ErrorResponse
	f.ApiGroup.DELETE("/users/:name",
		func(c *gin.Context) {
			b := f.dt.NewUser()
			b.Name = c.Param(`name`)
			f.Remove(c, b)
		})
}
