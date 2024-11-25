package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Serve static files (CSS, JS, etc.)
	r.Static("/static", "./static")

	// Load HTML templates
	r.LoadHTMLGlob("templates/*")

	// Home page
	r.GET("/index", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Handle CSV uploads
	r.POST("/upload", func(c *gin.Context) {
		gin.SetMode(gin.ReleaseMode)
		// Get files from form
		rolesFile, err := c.FormFile("roles.csv")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing roles file"})
			return
		}

		tasksFile, err := c.FormFile("tasks.csv")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tasks file"})
			return
		}

		devsFile, err := c.FormFile("developers.csv")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing developers file"})
			return
		}

		oncallsFile, err := c.FormFile("oncalls.csv")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing oncalls file"})
			return
		}

		leavesFile, err := c.FormFile("leaves.csv")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing leaves file"})
			return
		}

		// Save uploaded files temporarily
		tempFiles := make([]string, 5)
		uploadedFiles := []*multipart.FileHeader{rolesFile, tasksFile, devsFile, oncallsFile, leavesFile}

		for i, file := range uploadedFiles {
			tempFile := "temp_" + file.Filename
			if err := c.SaveUploadedFile(file, tempFile); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save uploaded file"})
				return
			}
			tempFiles[i] = tempFile
			defer os.Remove(tempFile)
		}

		// Load data from CSVs
		tasks, developers, roles, err := LoadFromCSV(tempFiles[0], tempFiles[1], tempFiles[2])
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Load oncalls and leaves
		oncalls, err := loadOncalls(tempFiles[3])
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		leaves, err := loadLeaves(tempFiles[4])
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		hasCyclicDependencies := func(tasks []*Task) bool {
			visited := make(map[string]bool)
			recStack := make(map[string]bool)

			for _, task := range tasks {
				if !visited[task.Name] {
					if detectCycle(task, visited, recStack, tasks) {
						return true
					}
				}
			}
			return false
		}

		// Check for cyclic dependencies
		if hasCyclicDependencies(tasks) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cyclic dependencies found"})
			return
		}

		// Create scheduler and process tasks
		scheduler := NewScheduler(tasks, developers, roles, oncalls, leaves)
		scheduler.Schedule(time.Now())

		// Return timeline data
		timelineData := processScheduleToTimelineData(scheduler)
		c.JSON(http.StatusOK, timelineData)
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}

type TimelineItem struct {
	ID      string `json:"id"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Content string `json:"content"`
}

func processScheduleToTimelineData(s *Scheduler) []TimelineItem {
	var items []TimelineItem

	// Process tasks into timeline items
	for _, task := range s.tasks {
		if task.StartTime.IsZero() || task.EndTime.IsZero() {
			continue // Skip unscheduled tasks
		}

		// Create timeline items for each developer assigned to the task
		for devName, startTime := range task.DevStartTimes {
			items = append(items, TimelineItem{
				ID:      fmt.Sprintf("task_%s_%s", task.Name, devName),
				Start:   startTime.Format("2006-01-02"),
				End:     task.EndTime.Format("2006-01-02"),
				Content: fmt.Sprintf("Task: %s (Assigned to: %s)", task.Name, devName),
			})
		}
	}

	// Add oncall periods
	for i, oncall := range s.oncalls {
		items = append(items, TimelineItem{
			ID:      fmt.Sprintf("oncall_%d", i),
			Start:   oncall.StartTime.Format("2006-01-02"),
			End:     oncall.EndTime.Format("2006-01-02"),
			Content: fmt.Sprintf("On-call: %s", oncall.DevName),
		})
	}

	// Add leave periods
	for i, leave := range s.leaves {
		items = append(items, TimelineItem{
			ID:      fmt.Sprintf("leave_%d", i),
			Start:   leave.StartTime.Format("2006-01-02"),
			End:     leave.EndTime.Format("2006-01-02"),
			Content: fmt.Sprintf("Leave: %s", leave.DevName),
		})
	}

	return items
}

func detectCycle(task *Task, visited, recStack map[string]bool, tasks []*Task) bool {
	visited[task.Name] = true
	recStack[task.Name] = true

	for _, depName := range task.Dependencies {
		// Find dependent task
		var depTask *Task
		for _, t := range tasks {
			if t.Name == depName {
				depTask = t
				break
			}
		}
		if depTask == nil {
			continue
		}

		if !visited[depTask.Name] {
			if detectCycle(depTask, visited, recStack, tasks) {
				return true
			}
		} else if recStack[depTask.Name] {
			return true
		}
	}

	recStack[task.Name] = false
	return false
}

func loadOncalls(filename string) ([]OnCall, error) {
	var oncalls []OnCall
	records, err := readCSV(filename)
	if err != nil {
		return nil, err
	}

	for _, record := range records[1:] { // Skip header
		startTime, _ := time.Parse("2006-01-02", record[1])
		endTime, _ := time.Parse("2006-01-02", record[2])
		oncalls = append(oncalls, OnCall{
			DevName:   record[0],
			StartTime: startTime,
			EndTime:   endTime,
		})
	}
	return oncalls, nil
}

func loadLeaves(filename string) ([]Leave, error) {
	var leaves []Leave
	records, err := readCSV(filename)
	if err != nil {
		return nil, err
	}

	for _, record := range records[1:] { // Skip header
		startTime, _ := time.Parse("2006-01-02", record[1])
		endTime, _ := time.Parse("2006-01-02", record[2])
		leaves = append(leaves, Leave{
			DevName:   record[0],
			StartTime: startTime,
			EndTime:   endTime,
		})
	}
	return leaves, nil
}

func LoadFromCSV(rolesFile, tasksFile, devsFile string) ([]*Task, []*Developer, map[string]*Role, error) {
	// Load Roles
	roles := make(map[string]*Role)
	if roleRecords, err := readCSV(rolesFile); err == nil {
		for _, record := range roleRecords[1:] { // Skip header
			availability, _ := strconv.ParseFloat(record[1], 64)
			roles[record[0]] = &Role{
				Name:                record[0],
				AvailabilityPercent: availability,
			}
		}
	} else {
		return nil, nil, nil, fmt.Errorf("error reading roles: %v", err)
	}

	// Load Tasks
	var tasks []*Task
	if taskRecords, err := readCSV(tasksFile); err == nil {
		for _, record := range taskRecords[1:] { // Skip header
			priority, _ := strconv.Atoi(record[2])
			effort, _ := strconv.ParseFloat(record[3], 64)
			parallel, _ := strconv.Atoi(record[4])
			dependencies := strings.Split(record[5], ",")
			if record[5] == "" {
				dependencies = []string{}
			}

			// Parse FE and QA required flags
			needsFE := false
			needsQA := false
			if len(record) > 6 {
				needsFE = record[6] == "true"
			}
			if len(record) > 7 {
				needsQA = record[7] == "true"
			}

			// Create main task
			taskName := record[0]
			mainTask := &Task{
				Name:           taskName,
				TaskType:       record[1],
				Priority:       priority,
				ParallelFactor: parallel,
				Dependencies:   dependencies,
				IsCompleted:    false,
			}

			// Increase effort by (10*parallelFactor + 40)%
			effortIncrease := 1.0 + float64(10*parallel+40)/100.0
			mainTask.Effort = math.Round(effort * effortIncrease)

			tasks = append(tasks, mainTask)

			// Add Frontend task if flag is true
			var feTaskName string
			if needsFE {
				feTaskName = taskName + "_Frontend"
				feTask := &Task{
					Name:           feTaskName,
					TaskType:       "Frontend",
					Priority:       priority,
					Effort:         math.Round(effort * 0.25 * effortIncrease),
					ParallelFactor: 1,
					Dependencies:   []string{taskName},
					IsCompleted:    false,
				}
				tasks = append(tasks, feTask)
			}

			// Add QA task if both flags are true
			if needsQA {
				qaTaskName := taskName + "_QA"
				dependencies := []string{taskName}
				if needsFE {
					dependencies = append(dependencies, feTaskName)
				}
				qaTask := &Task{
					Name:           qaTaskName,
					TaskType:       "QA",
					Priority:       priority,
					Effort:         math.Round(effort * 0.25 * effortIncrease),
					ParallelFactor: 1,
					Dependencies:   dependencies,
					IsCompleted:    false,
				}
				tasks = append(tasks, qaTask)
			}
		}
	} else {
		return nil, nil, nil, fmt.Errorf("error reading tasks: %v", err)
	}

	// Load Developers
	var developers []*Developer
	if devRecords, err := readCSV(devsFile); err == nil {
		for _, record := range devRecords[1:] { // Skip header
			taskTypes := strings.Split(record[2], ",")
			developers = append(developers, &Developer{
				Name:      record[0],
				Role:      record[1],
				TaskTypes: taskTypes,
			})
		}
	} else {
		return nil, nil, nil, fmt.Errorf("error reading developers: %v", err)
	}

	return tasks, developers, roles, nil
}

func readCSV(filename string) ([][]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return records, nil
}
