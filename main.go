package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Config struct {
	ADMIN    string `json:"admin"`
	PASSWORD string `json:"password"`
	ROOT_URL string `json:"root_url"`
	RANDOM_KEY string `json:"random_key"`
}

type Worker struct {
	ID string `json:"id"`
	IP string `json:"ip"`
}

var job_running bool = false
var total_requests int64 = 0
var job_start_time int64 = 0
var quit bool = false

func httpClient() *http.Client {
	client := &http.Client{Timeout: 10 * time.Second}
	return client
}

func sendRequest(client *http.Client, url string, method string) []byte {
    values := map[string]string{"foo": "baz"}
	jsonData, err := json.Marshal(values)

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Error Occurred. %+v", err)
	}

	response, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request to API endpoint. %+v", err)
	}

	// Close the connection to reuse it
	response.Body.Close()
	return nil
}


func job(url string, c *http.Client) {
	//Do the job
	fmt.Printf("Job started for %s\n", url)
	for {
		if quit {
			//Quit the job if quit flag is set to true
			break
		}
			//Make the request
			sendRequest(c, url, http.MethodGet )
			//Increment total requests
			total_requests++
		}
		
}

func initJob(url string, workers string) {
	c := httpClient()
	//Reset quit flag
	quit = false
	//Convert workers to int
	workerInt, _ := strconv.Atoi(workers)
	//Set the number of goroutines to the number of workers
		for i := 0; i < workerInt; i++ {
			go job(url, c)
		}
		//Set job is running
		job_running = true
		//Set job start time
		job_start_time = time.Now().Unix()

		go System()
}

// Print system resource usage every 2 seconds.
func System() {
	mem := &runtime.MemStats{}
 
	for {
		cpu := runtime.NumCPU()
		log.Println("CPU:", cpu)
 
		rot := runtime.NumGoroutine()
		log.Println("Goroutine:", rot)
 
		// Byte
		runtime.ReadMemStats(mem)
		log.Println("Memory:", mem.Alloc)
 
		time.Sleep(2 * time.Second)
		log.Println("-------")
	}
}

func main() {
	//Make Workers Directory
	os.MkdirAll("workers", 0777)

	//Start the server
	router := gin.Default()
	
	router.POST("/new-job", func(c *gin.Context) {
		//Check if job is running
		if job_running {
			c.JSON(http.StatusOK, gin.H{
				"message": "Job is already running",
			})
			return
		}
		//Start the job	
		initJob(c.PostForm("url"), c.PostForm("workers"))
		//Return the job success
		c.JSON(http.StatusOK, gin.H{
			"message": "new job started",
		})
	})

	router.POST("/register-worker", func(c *gin.Context) {
		//Get the config
		config := Config{}
		//Get the config file
		file, _ := ioutil.ReadFile("config.json")
		//Unmarshal the config file
		json.Unmarshal(file, &config)

		//Check the key
		if c.PostForm("key") != config.RANDOM_KEY {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "wrong key",
			})
			return
		}

		//Generate Worker ID
		worker := Worker{
			ID: uuid.New().String(),
			IP: c.ClientIP(),
		}

		//Save the worker
		file, _ = json.Marshal(worker)
		if err := ioutil.WriteFile("workers/" + worker.ID, file, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "error saving worker",
			})
			return
		}

		//Return the worker ID
		c.JSON(http.StatusOK, gin.H{
			"message": "worker registered",
			"workerID": worker.ID,
		})
	})

	router.GET("/health-check", func(c *gin.Context) {
		//Ping pong
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.GET("/job-status", func (c *gin.Context) {
		//Check if a job is running
		if(job_running) {
			//If a job is running return the job status
			c.JSON(http.StatusOK, gin.H{
				"message": "job is running",
				"total_requests": total_requests,
				"requests_per_second": total_requests / (time.Now().Unix() - job_start_time),
			})
			return
		}
			//If no job is running return the job status
			c.JSON(http.StatusOK, gin.H{
				"message": "job is not running",
			})
	})

	router.GET("/stop-job", func (c *gin.Context) {
		//Check if a job is running
		if condition := job_running; condition {
			quit = true
			job_running = false
			c.JSON(http.StatusOK, gin.H{
				"message": "job stopped",
			})
			return
		}
		//If no job is running return the job status
		c.JSON(http.StatusOK, gin.H{
			"message": "job is not running",
		})
	})

	//Generate Config File on first run
	router.POST("/quick-start", func(c *gin.Context) {
		//Check if config file exists
		if _, err := os.Stat("config.json"); err == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Config file already exists. Please delete it and try again.",
			})
			return
		  }

		//Generate Config Struct
		data := Config{
			c.PostForm("admin"),
			c.PostForm("password"),
			c.Request.Host,
			uuid.New().String(),
		}

		//Convert to JSON
		b, _ := json.Marshal(data)

		//Write to file
		ioutil.WriteFile("config.json", b, 0644)

		//Return Success
		c.JSON(http.StatusOK, data)
	})
	
	router.Run() // listen and serve on port 8080
}