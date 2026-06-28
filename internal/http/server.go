package http_handler

import "log"
import "net/http"
import "encoding/json"
import "github.com/motangpuar/o2-ims-worker/internal/db"

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
	if r.Method == http.MethodPost {
		defer r.Body.Close()
		var pipe pipeLine 
		err := json.NewDecoder(r.Body).Decode(&pipe)

		if err != nil {
			log.Printf("Broken Format !!!!")
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("[HTTP] Received data %+v", pipe)
		filedata.AddItemToFile(pipe.IP, pipe.Mac, pipe.OS)
	} else if r.Method == http.MethodGet {
		log.Printf("[HTTP] Request for pipeline: %s", r.URL.Path)
		fd := filedata.Gather() 
		log.Printf("[HTTP] Fetch Gateher: %s", fd.Clients)
	} else {
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
