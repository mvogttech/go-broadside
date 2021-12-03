package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
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
var quit = make(chan bool)


func job(url string) {
	//Do the job
	fmt.Printf("Job started for %s\n", url)
	for {
		select {
        case <- quit:
            return
        default:
			resp, err := http.Get(url)
			if err != nil {
				fmt.Printf(err.Error())
				quit <- true
				break
			}
			total_requests++
			fmt.Printf(resp.Status)
			resp.Body.Close()
		}
		
	}
}

func main() {
	//Make Workers Directory
	os.MkdirAll("workers", 0777)

	//Start the server
	router := gin.Default()
	
	router.POST("/new-job", func(c *gin.Context) {
		//Get Processors
		CPUs := runtime.NumCPU()
		for i := 0; i < (CPUs - 1); i++ {
			go job(c.PostForm("url"))
		}
		job_running = true
		job_start_time = time.Now().Unix()
		c.JSON(http.StatusOK, gin.H{
			"message": "new job started",
		})
	})

	router.POST("/register-worker", func(c *gin.Context) {
		//Get the config
		config := Config{}
		file, _ := ioutil.ReadFile("config.json")
		json.Unmarshal(file, &config)

		//Check the password
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
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.GET("/job-status", func (c *gin.Context) {
		//Check if a job is running
		if(job_running) {
			c.JSON(http.StatusOK, gin.H{
				"message": "job is running",
				"total_requests": total_requests,
				"requests_per_second": total_requests / (time.Now().Unix() - job_start_time),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"message": "job is not running",
			})
		}
	})

	router.GET("/stop-job", func (c *gin.Context) {
		quit <- true
		job_running = false
		total_requests = 0
		c.JSON(http.StatusOK, gin.H{
			"message": "job stopped",
		})
	})

	//Generate Config File on first run
	router.POST("/quick-start", func(c *gin.Context) {
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