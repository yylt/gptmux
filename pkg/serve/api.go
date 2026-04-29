package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type apiMerlinUserRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

func (ca *Controller) API(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")
	switch {
	case path == "/merlin/users":
		ca.handleMerlinUsers(w, r)
	case strings.HasPrefix(path, "/merlin/users/"):
		ca.handleMerlinUser(w, r, strings.TrimPrefix(path, "/merlin/users/"))
	default:
		writeJSON(w, http.StatusNotFound, apiErrorResponse{Error: fmt.Sprintf("unknown api path %q", r.URL.Path)})
	}
}

func (ca *Controller) handleMerlinUsers(w http.ResponseWriter, r *http.Request) {
	merlinModel := ca.Merlin()
	if merlinModel == nil {
		writeJSON(w, http.StatusNotFound, apiErrorResponse{Error: "merlin is not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, merlinModel.ListUsers())
	case http.MethodPost:
		var req apiMerlinUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{Error: "invalid json body"})
			return
		}
		if err := merlinModel.AddUser(strings.TrimSpace(req.Name), req.Password); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
			return
		}
		if err := ca.SaveConfig(); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiErrorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, merlinModel.ListUsers())
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ca *Controller) handleMerlinUser(w http.ResponseWriter, r *http.Request, name string) {
	merlinModel := ca.Merlin()
	if merlinModel == nil {
		writeJSON(w, http.StatusNotFound, apiErrorResponse{Error: "merlin is not configured"})
		return
	}
	username := strings.TrimSpace(name)
	if username == "" {
		writeJSON(w, http.StatusBadRequest, apiErrorResponse{Error: "user name is required"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var req apiMerlinUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{Error: "invalid json body"})
			return
		}
		if err := merlinModel.UpdateUser(username, req.Password); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
			return
		}
		if err := ca.SaveConfig(); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiErrorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, merlinModel.ListUsers())
	case http.MethodDelete:
		if err := merlinModel.DeleteUser(username); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{Error: err.Error()})
			return
		}
		if err := ca.SaveConfig(); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiErrorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, merlinModel.ListUsers())
	default:
		w.Header().Set("Allow", "PUT, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
