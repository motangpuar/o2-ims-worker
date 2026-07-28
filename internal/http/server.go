package http_handler

import "context"
import "log"
import "time"
import "net/http"
import "encoding/json"
import "github.com/motangpuar/o2-ims-worker/internal/db"
import "github.com/motangpuar/o2-ims-worker/internal/inventory"
import "github.com/motangpuar/o2-ims-worker/internal/ansible"
import "github.com/motangpuar/o2-ims-worker/internal/kubernetes"
import "slices"

type pipeLine struct {
	Name string `json:"name"`
	Mac string `json:"mac"`
	IP string `json:"ip"`
	OS string `json:"os"`
}

var httpContext *context.Context

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

func handleInventory(w http.ResponseWriter, r *http.Request){
	log.Printf("[HTTP] Request for inventories")
	inventories := inventory.FetchMachines()
	
	err := json.NewEncoder(w).Encode(inventories)
	if err != nil {
		log.Printf("[HTTP] Error...")
		return
	}
	log.Printf("[HTTP] Inventories, %v", inventories)
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

func handleCallback(w http.ResponseWriter, r *http.Request){
	log.Printf("[HTTP] Request arrive for Callback...")
	queryParams := r.URL.Query()

	macQuery := queryParams.Get("mac")

	if macQuery == "" {
		log.Printf("[HTTP] Machines %v", inventory.FetchInstalledMachines())
	} else {
		err := inventory.LogInstalledMachine(macQuery, "", time.Now())
		if err != nil {
			http.Error(w, "Error "+err.Error(), http.StatusBadRequest)
		} else {
			w.Write([]byte("Success!"))
		}
	}

}

func handleAnsible(w http.ResponseWriter, r *http.Request){
	log.Printf("[HTTP] Request arrive for Ansible")

	queryParams := r.URL.Query()
	macQuery := queryParams.Get("mac")

	if macQuery == "" {
		log.Printf("[HTTP] Ansible Options")
	} else {
		value, exist := filedata.Gather().Clients[macQuery]
		username := value.OSType()
		if exist != true {
			return
		}
		ansible_worker.Populate(value.OfferIP(), macQuery, username)
	}
}

type clusterJson struct {
	Info any
	Nodes any
}

func handleKubernetes(w http.ResponseWriter, r *http.Request){
	ansibleInfo := ansible_worker.GetInfo()
	kc, err := kubeclient.New(ansibleInfo.KubeconfigPath)
	if err != nil {
		log.Printf("[HTTP] Failed to established kubernetes connection")
		return
	}

	info,err := kc.ClusterInfo(*httpContext)
	if err != nil {
		log.Printf("[HTTP] Failed to query kubernetes") 
		return
	}

	log.Printf("[HTTP] Kubernetes Context: version %s, nodes %d, namespaces %v", info.ServerVersion, info.NodeCount, info.Namespaces)

	nodes,err := kc.Nodes(*httpContext)
	if err != nil {
		log.Printf("[HTTP] Failed to query nodes")
		return
	}

	for _, n := range(nodes) {
		log.Printf("[HTTP] node %s, status %s, cpu %s, mem %s, ip: %s",
		n.Name, n.Status, n.CPUAllocatable, n.MemoryAllocatable, n.InternalIP)
	}
	
	//var payload []any
	//responseBody := append(payload, info, nodes)

	responseBody := clusterJson{
		Info: info,
		Nodes: nodes,
	}
	
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(responseBody)
	if err != nil {
		log.Printf("Failed to send Response")
		http.Error(w, "Error Detected", http.StatusInternalServerError)
		return
	}
}

func Serve(ctx context.Context) {
	// Populate global context
	httpContext = &ctx

	dir := "assets/http/"
	mux := http.NewServeMux()
	filehandler := http.StripPrefix("/", http.FileServer(http.Dir(dir)))
	mux.HandleFunc("/", logFileServer(filehandler))
	mux.HandleFunc("/pipeline", handlePipeline)
	mux.HandleFunc("/test", handleTest)
	mux.HandleFunc("/inventories", handleInventory)
	mux.HandleFunc("/callback", handleCallback)
	mux.HandleFunc("/ansible", handleAnsible)
	mux.HandleFunc("/kubernetes", handleKubernetes)
	log.Fatal(http.ListenAndServe(":8033", mux))
}
