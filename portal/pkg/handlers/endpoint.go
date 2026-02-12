package handlers

import (
	"encoding/hex"
	"strconv"

	endpointadaptors "github.com/ashupednekar/litefunctions/portal/internal/endpoint/adaptors"
	"github.com/ashupednekar/litefunctions/portal/pkg/state"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type EndpointHandlers struct {
	state *state.AppState
}

func NewEndpointHandlers(s *state.AppState) *EndpointHandlers {
	return &EndpointHandlers{state: s}
}

func (h *EndpointHandlers) ListEndpoints(c *gin.Context) {
	projectUUID := c.MustGet("projectUUID").(pgtype.UUID)
	search := c.Query("search")
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	q := endpointadaptors.New(h.state.DBPool)
	eps, err := q.ListEndpointsSearch(c.Request.Context(), endpointadaptors.ListEndpointsSearchParams{
		ProjectID: projectUUID,
		Column2:   search,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "database error"})
		return
	}

	type endpointResponse struct {
		ID           string `json:"ID"`
		Name         string `json:"Name"`
		Method       string `json:"Method"`
		Scope        string `json:"Scope"`
		FunctionName string `json:"FunctionName"`
	}

	result := make([]endpointResponse, 0, len(eps))
	for _, e := range eps {
		result = append(result, endpointResponse{
			ID:           hex.EncodeToString(e.ID.Bytes[:]),
			Name:         e.Name,
			Method:       e.Method,
			Scope:        e.Scope,
			FunctionName: e.FunctionName,
		})
	}

	c.JSON(200, result)
}

func (h *EndpointHandlers) GetEndpoint(c *gin.Context) {
	epIDHex := c.Param("epID")
	epIDBytes, err := hex.DecodeString(epIDHex)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid endpoint id"})
		return
	}
	var epUUID pgtype.UUID
	copy(epUUID.Bytes[:], epIDBytes)
	epUUID.Valid = true

	q := endpointadaptors.New(h.state.DBPool)
	ep, err := q.GetEndpointByID(c.Request.Context(), epUUID)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	c.JSON(200, ep)
}

func (h *EndpointHandlers) UpdateEndpoint(c *gin.Context) {
	epIDHex := c.Param("epID")
	epIDBytes, err := hex.DecodeString(epIDHex)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid endpoint id"})
		return
	}
	var epUUID pgtype.UUID
	copy(epUUID.Bytes[:], epIDBytes)
	epUUID.Valid = true

	var req struct {
		Method string `json:"method"`
		Scope  string `json:"scope"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	q := endpointadaptors.New(h.state.DBPool)
	ep, err := q.UpdateEndpointMethodScope(c.Request.Context(), endpointadaptors.UpdateEndpointMethodScopeParams{
		ID:     epUUID,
		Method: req.Method,
		Scope:  req.Scope,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "database error"})
		return
	}

	c.JSON(200, ep)
}

