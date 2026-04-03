package get_example

import (
	"encoding/json"
	"net/http"

	"github.com/Gen-Do/service-configs/internal/generated/server/api"
)

func Handler(w http.ResponseWriter, r *http.Request, params api.GetExampleParams) {
	message := "Hello, " + params.Name + "!"
	response := api.ExampleResponse{
		Message: &message,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
