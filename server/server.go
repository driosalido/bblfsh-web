package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"gopkg.in/bblfsh/client-go.v2"
	"gopkg.in/bblfsh/sdk.v1/protocol"
	"gopkg.in/bblfsh/sdk.v1/uast"
)

type Server struct {
	client  *bblfsh.Client
	version string
}

func New(addr string, version string) (*Server, error) {
	client, err := bblfsh.NewClient(addr)
	if err != nil {
		return nil, err
	}

	return &Server{client, version}, nil
}

type request struct {
	ServerURL string `json:"server_url"`
}

type parseRequest struct {
	request
	Language string `json:"language"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

func (s *Server) HandleParse(ctx *gin.Context) {
	var req parseRequest
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, jsonError("unable to read request: %s", err))
		return
	}

	cli, err := s.clientForRequest(req.request)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, jsonError("error starting client: %s", err))
		return
	}

	resp, err := cli.NewParseRequest().
		Language(req.Language).
		Filename(req.Filename).
		Content(req.Content).
		Do()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, jsonError("error parsing UAST: %s", err))
		return
	}

	ctx.JSON(toHTTPStatus(resp.Status), (*ParseResponse)(resp))
}

func (s *Server) clientForRequest(req request) (*bblfsh.Client, error) {
	if req.ServerURL == "" {
		return s.client, nil
	}

	return bblfsh.NewClient(req.ServerURL)
}

func (s *Server) LoadGist(ctx *gin.Context) {
	gistUrl := "https://gist.githubusercontent.com/" + ctx.Query("url")

	resp, err := http.Get(gistUrl)
	if err != nil {
		ctx.JSON(http.StatusNotFound, jsonError("Gist not found: %s", err))
		return
	}
	defer resp.Body.Close()

	gistContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, jsonError("Could not read gist: %s", err))
		return
	}

	ctx.String(resp.StatusCode, string(gistContent))
}

type versionRequest struct {
	request
}

func (s *Server) Version(ctx *gin.Context) {
	req := versionRequest{}
	req.ServerURL = ctx.Query("server_url")

	cli, err := s.clientForRequest(req.request)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, jsonError("error starting client: %s", err))
		return
	}

	resp, err := cli.NewVersionRequest().Do()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, jsonError("error getting server version: %s", err))
	}

	ctx.JSON(toHTTPStatus(resp.Status), map[string]string{
		"dashboard": s.version,
		"server":    resp.Version,
	})
}

func (s *Server) ListDrivers(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, driverList)
}

func toHTTPStatus(status protocol.Status) int {
	switch status {
	case protocol.Ok:
		return http.StatusOK
	case protocol.Error:
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}

func jsonError(msg string, args ...interface{}) gin.H {
	return gin.H{
		"status": protocol.Fatal,
		"errors": []gin.H{
			gin.H{
				"message": fmt.Sprintf(msg, args...),
			},
		},
	}
}

type ParseResponse protocol.ParseResponse

func (r *ParseResponse) MarshalJSON() ([]byte, error) {
	resp := struct {
		*protocol.ParseResponse
		UAST *Node `json:"uast"`
	}{
		(*protocol.ParseResponse)(r),
		(*Node)(r.UAST),
	}

	return json.Marshal(resp)
}

type Node uast.Node

func (n *Node) MarshalJSON() ([]byte, error) {
	var nodes = make([]*Node, len(n.Children))
	for i, n := range n.Children {
		nodes[i] = (*Node)(n)
	}

	var roles = make([]string, len(n.Roles))
	for i, r := range n.Roles {
		roles[i] = r.String()
	}

	node := struct {
		*uast.Node
		Roles    []string
		Children []*Node
	}{
		(*uast.Node)(n),
		roles,
		nodes,
	}

	return json.Marshal(node)
}
