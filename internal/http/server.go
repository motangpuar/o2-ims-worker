package http_handler

import "log"
import "net/http"
import "encoding/json"
import "github.com/motangpuar/o2-ims-worker/internal/db"
import "slices"

type pipeLine struct {
	Name string `json:"name"`
	Mac string `json:"mac"`
	IP string `json:"ip"`
	OS string `json:"os"`
}

func handleTest(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Request arrive for Test Path...")
	w.Write([]byte("This is the test path...\n"))
}

func logFileServer(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[HTTP] Request for file: %s", r.URL.Path)
		next.ServeHTTP(w, r)
	}
}

func handlePipeline(w http.ResponseWriter, r *http.Request) {
	fd := filedata.Gather() 
	responseBody := fd.Clients

	switch response := r.Method; response {
	case http.MethodPost:
		defer r.Body.Close()
		var pipe pipeLine 
		err := json.NewDecoder(r.Body).Decode(&pipe)

		if err != nil {
			log.Printf("Broken Format !!!!")
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		
		log.Printf("[HTTP] Received data %+v", pipe)
		validOSTypes := []string{"debian", "centos", "ubuntu"}
		if  !slices.Contains(validOSTypes, pipe.OS) {
			log.Printf("No OS Type like that")
			http.Error(w, "OS Type Not Exist: "+pipe.OS+". Only valid options are (debian, centos, ubuntu)", http.StatusBadRequest)
			return
		}

		if responseBody[pipe.Mac] != nil {
			log.Printf("[HTTP] Entry Exist")
			http.Error(w, "Entry Exist: "+pipe.Mac, http.StatusBadRequest)
			return
		} else {
			for _,c := range responseBody{
				if c.OfferIP() == pipe.IP {
					log.Printf("[HTTP] IP Exist")
					http.Error(w, "IP Exist: "+pipe.IP, http.StatusBadRequest)
					return
				}
			}
		}
		
		filedata.AddItemToFile(pipe.IP, pipe.Mac, pipe.OS)
	case http.MethodGet:
		log.Printf("[HTTP] Request for pipeline: %s", r.URL.Path)
		jsonPayload := make(map[string]any)

		for m,c := range responseBody {
			jsonPayload[m] = c.ToMap()
		}
		
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(jsonPayload)
		if err != nil {
			log.Printf("Failed to send Response")
			http.Error(w, "Error Detected", http.StatusInternalServerError)
			return
		}
	default:
		log.Printf("[HTTP] Bad Request: %s", r.URL.Path)
		return
	}
}

func Serve() {
	dir := "assets/http/"
	mux := http.NewServeMux()
	filehandler := http.StripPrefix("/", http.FileServer(http.Dir(dir)))
	mux.HandleFunc("/", logFileServer(filehandler))
	mux.HandleFunc("/pipeline", handlePipeline)
	mux.HandleFunc("/test", handleTest)
	log.Fatal(http.ListenAndServe(":8033", mux))
}
