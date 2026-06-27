package http_handler

import "log"
import "net/http"

func handleTest(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Request arrive for Test Path...")
	w.Write([]byte("This is the test path..."))
}

func logFileServer(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[HTTP] Request for file: %s", r.URL.Path)
		next.ServeHTTP(w, r)
	}
}

func Serve() {
	dir := "assets/http/"
	mux := http.NewServeMux()
	filehandler := http.StripPrefix("/", http.FileServer(http.Dir(dir)))
	mux.HandleFunc("/", logFileServer(filehandler))

	mux.HandleFunc("/test", handleTest)
	log.Fatal(http.ListenAndServe(":8033", mux))
}
