package frontend

import (
	"fmt"
	"net"
	"net/http"

	"github.com/VictorLowther/jsonpatch2"
	"github.com/digitalrebar/provision/backend"
	"github.com/digitalrebar/provision/models"
	"github.com/gin-gonic/gin"
)

// ReservationResponse returned on a successful GET, PUT, PATCH, or POST of a single reservation
// swagger:response
type ReservationResponse struct {
	// in: body
	Body *models.Reservation
}

// ReservationsResponse returned on a successful GET of all the reservations
// swagger:response
type ReservationsResponse struct {
	//in: body
	Body []*models.Reservation
}

// ReservationBodyParameter used to inject a Reservation
// swagger:parameters createReservation putReservation
type ReservationBodyParameter struct {
	// in: body
	// required: true
	Body *models.Reservation
}

// ReservationPatchBodyParameter used to patch a Reservation
// swagger:parameters patchReservation
type ReservationPatchBodyParameter struct {
	// in: body
	// required: true
	Body jsonpatch2.Patch
}

// ReservationPathParameter used to address a Reservation in the path
// swagger:parameters putReservations getReservation putReservation patchReservation deleteReservation headReservation
type ReservationPathParameter struct {
	// in: path
	// required: true
	// swagger:strfmt ipv4
	Address string `json:"address"`
}

// ReservationListPathParameter used to limit lists of Reservation by path options
// swagger:parameters listReservations listStatsReservations
type ReservationListPathParameter struct {
	// in: query
	Offest int `json:"offset"`
	// in: query
	Limit int `json:"limit"`
	// in: query
	Available string
	// in: query
	Valid string
	// in: query
	ReadOnly string
	// in: query
	Addr string
	// in: query
	Token string
	// in: query
	Strategy string
	// in: query
	NextServer string
}

func (f *Frontend) InitReservationApi() {
	// swagger:route GET /reservations Reservations listReservations
	//
	// Lists Reservations filtered by some parameters.
	//
	// This will show all Reservations by default.
	//
	// You may specify:
	//    Offset = integer, 0-based inclusive starting point in filter data.
	//    Limit = integer, number of items to return
	//
	// Functional Indexs:
	//    Addr = IP Address
	//    Token = string
	//    Strategy = string
	//    NextServer = IP Address
	//    Available = boolean
	//    Valid = boolean
	//    ReadOnly = boolean
	//
	// Functions:
	//    Eq(value) = Return items that are equal to value
	//    Lt(value) = Return items that are less than value
	//    Lte(value) = Return items that less than or equal to value
	//    Gt(value) = Return items that are greater than value
	//    Gte(value) = Return items that greater than or equal to value
	//    Between(lower,upper) = Return items that are inclusively between lower and upper
	//    Except(lower,upper) = Return items that are not inclusively between lower and upper
	//
	// Example:
	//    Name=fred - returns items named fred
	//    Name=Lt(fred) - returns items that alphabetically less than fred.
	//    Name=Lt(fred)&Available=true - returns items with Name less than fred and Available is true
	//
	// Responses:
	//    200: ReservationsResponse
	//    401: NoContentResponse
	//    403: NoContentResponse
	//    406: ErrorResponse
	f.ApiGroup.GET("/reservations",
		func(c *gin.Context) {
			f.List(c, &backend.Reservation{})
		})

	// swagger:route HEAD /reservations Reservations listStatsReservations
	//
	// Stats of the List Reservations filtered by some parameters.
	//
	// This will return headers with the stats of the list.
	//
	// You may specify:
	//    Offset = integer, 0-based inclusive starting point in filter data.
	//    Limit = integer, number of items to return
	//
	// Functional Indexs:
	//    Addr = IP Address
	//    Token = string
	//    Strategy = string
	//    NextServer = IP Address
	//    Available = boolean
	//    Valid = boolean
	//    ReadOnly = boolean
	//
	// Functions:
	//    Eq(value) = Return items that are equal to value
	//    Lt(value) = Return items that are less than value
	//    Lte(value) = Return items that less than or equal to value
	//    Gt(value) = Return items that are greater than value
	//    Gte(value) = Return items that greater than or equal to value
	//    Between(lower,upper) = Return items that are inclusively between lower and upper
	//    Except(lower,upper) = Return items that are not inclusively between lower and upper
	//
	// Example:
	//    Name=fred - returns items named fred
	//    Name=Lt(fred) - returns items that alphabetically less than fred.
	//    Name=Lt(fred)&Available=true - returns items with Name less than fred and Available is true
	//
	// Responses:
	//    200: NoContentResponse
	//    401: NoContentResponse
	//    403: NoContentResponse
	//    406: ErrorResponse
	f.ApiGroup.HEAD("/reservations",
		func(c *gin.Context) {
			f.ListStats(c, &backend.Reservation{})
		})

	// swagger:route POST /reservations Reservations createReservation
	//
	// Create a Reservation
	//
	// Create a Reservation from the provided object
	//
	//     Responses:
	//       201: ReservationResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       409: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.POST("/reservations",
		func(c *gin.Context) {
			b := &backend.Reservation{}
			f.Create(c, b)
		})

	// swagger:route GET /reservations/{address} Reservations getReservation
	//
	// Get a Reservation
	//
	// Get the Reservation specified by {address} or return NotFound.
	//
	//     Responses:
	//       200: ReservationResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	f.ApiGroup.GET("/reservations/:address",
		func(c *gin.Context) {
			ip := net.ParseIP(c.Param(`address`))
			if ip == nil {
				c.JSON(http.StatusBadRequest,
					models.NewError(c.Request.Method, http.StatusBadRequest,
						fmt.Sprintf("reservation get: address not valid: %v", c.Param(`address`))))
				return
			}
			f.Fetch(c, &backend.Reservation{}, models.Hexaddr(ip))
		})

	// swagger:route HEAD /reservations/{address} Reservations headReservation
	//
	// See if a Reservation exists
	//
	// Return 200 if the Reservation specifiec by {address} exists, or return NotFound.
	//
	//     Responses:
	//       200: NoContentResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: NoContentResponse
	f.ApiGroup.HEAD("/reservations/:address",
		func(c *gin.Context) {
			f.Exists(c, &backend.Reservation{}, c.Param(`address`))
		})

	// swagger:route PATCH /reservations/{address} Reservations patchReservation
	//
	// Patch a Reservation
	//
	// Update a Reservation specified by {address} using a RFC6902 Patch structure
	//
	//     Responses:
	//       200: ReservationResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	//       406: ErrorResponse
	//       409: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.PATCH("/reservations/:address",
		func(c *gin.Context) {
			ip := net.ParseIP(c.Param(`address`))
			if ip == nil {
				c.JSON(http.StatusBadRequest,
					models.NewError(c.Request.Method, http.StatusBadRequest,
						fmt.Sprintf("reservation get: address not valid: %v", c.Param(`address`))))
				return
			}
			f.Patch(c, &backend.Reservation{}, models.Hexaddr(ip))
		})

	// swagger:route PUT /reservations/{address} Reservations putReservation
	//
	// Put a Reservation
	//
	// Update a Reservation specified by {address} using a JSON Reservation
	//
	//     Responses:
	//       200: ReservationResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	//       409: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.PUT("/reservations/:address",
		func(c *gin.Context) {
			ip := net.ParseIP(c.Param(`address`))
			if ip == nil {
				c.JSON(http.StatusBadRequest,
					models.NewError(c.Request.Method, http.StatusBadRequest,
						fmt.Sprintf("reservation put: address not valid: %v", c.Param(`address`))))
				return
			}
			f.Update(c, &backend.Reservation{}, models.Hexaddr(ip))
		})

	// swagger:route DELETE /reservations/{address} Reservations deleteReservation
	//
	// Delete a Reservation
	//
	// Delete a Reservation specified by {address}
	//
	//     Responses:
	//       200: ReservationResponse
	//       400: ErrorResponse
	//       401: NoContentResponse
	//       403: NoContentResponse
	//       404: ErrorResponse
	//       422: ErrorResponse
	f.ApiGroup.DELETE("/reservations/:address",
		func(c *gin.Context) {
			addr := net.ParseIP(c.Param(`address`))
			if addr == nil {
				c.JSON(http.StatusBadRequest,
					models.NewError(c.Request.Method, http.StatusBadRequest,
						fmt.Sprintf("reservation delete: address not valid: %v", c.Param(`address`))))
				return
			}
			f.Remove(c, &backend.Reservation{}, models.Hexaddr(addr))
		})
}
