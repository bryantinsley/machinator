package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Dependency struct {
	DependsOnID string `json:"depends_on_id"`
}

type Task struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Status       string       `json:"status"`
	Description  string       `json:"description,omitempty"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
}

func getStateFile() string {
	if f := os.Getenv("MOCK_BD_FILE"); f != "" {
		return f
	}
	return ".mock-bd.json"
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: bd <command>")
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--sandbox" {
		args = args[1:]
	}

	if len(args) == 0 {
		return
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "import":
		handleImport(cmdArgs)
	case "list":
		handleList(cmdArgs)
	case "update":
		handleUpdate(cmdArgs)
	case "close":
		handleClose(cmdArgs)
	case "ready":
		handleReady(cmdArgs)
	case "show":
		handleShow(cmdArgs)
	case "init", "create":
		// No-op
	default:
		// Ignore unknown or just exit 0 for compatibility
	}
}

func loadState() []Task {
	data, err := os.ReadFile(getStateFile())
	if err != nil {
		return []Task{}
	}
	var tasks []Task
	json.Unmarshal(data, &tasks)
	return tasks
}

func saveState(tasks []Task) {
	data, _ := json.MarshalIndent(tasks, "", "  ")
	os.WriteFile(getStateFile(), data, 0644)
}

func handleImport(args []string) {
	var file string
	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) {
			file = args[i+1]
			break
		}
	}
	if file == "" {
		return
	}

	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening import file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	var tasks []Task
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var t Task
		if err := json.Unmarshal(scanner.Bytes(), &t); err == nil {
			tasks = append(tasks, t)
		}
	}
	saveState(tasks)
}

func handleList(args []string) {
	tasks := loadState()
	// Always output JSON
	data, _ := json.Marshal(tasks)
	fmt.Println(string(data))
}

func handleReady(args []string) {
	tasks := loadState()

	// Create a map of open tasks for quick lookup
	openTasks := make(map[string]bool)
	for _, t := range tasks {
		if t.Status == "open" || t.Status == "in_progress" {
			openTasks[t.ID] = true
		}
	}

	var readyTasks []Task
	for _, t := range tasks {
		if t.Status != "open" {
			continue
		}

		isBlocked := false
		for _, dep := range t.Dependencies {
			// If dependency is open/in_progress, then it blocks this task
			if openTasks[dep.DependsOnID] {
				isBlocked = true
				break
			}
		}

		if !isBlocked {
			readyTasks = append(readyTasks, t)
		}
	}
	data, _ := json.Marshal(readyTasks)
	fmt.Println(string(data))
}

func handleShow(args []string) {
	if len(args) < 1 {
		return
	}
	id := args[0]
	// Handle --json flag if present
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else if !strings.HasPrefix(arg, "-") {
			id = arg
		}
	}

	tasks := loadState()
	for _, t := range tasks {
		if t.ID == id {
			if jsonOutput {
				data, _ := json.Marshal(t)
				fmt.Println(string(data))
			} else {
				fmt.Printf("Task %s: %s\n", t.ID, t.Title)
				fmt.Printf("Status: %s\n", t.Status)
				fmt.Printf("Description: %s\n", t.Description)
			}
			return
		}
	}
}

func handleUpdate(args []string) {
	if len(args) < 1 {
		return
	}
	id := args[0]
	var status string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--status=") {
			status = strings.TrimPrefix(arg, "--status=")
		}
	}

	if status == "" {
		return
	}

	tasks := loadState()
	for i := range tasks {
		if tasks[i].ID == id {
			tasks[i].Status = status
			break
		}
	}
	saveState(tasks)
}

func handleClose(args []string) {
	if len(args) < 1 {
		return
	}
	id := args[0]
	tasks := loadState()
	for i := range tasks {
		if tasks[i].ID == id {
			tasks[i].Status = "closed"
			break
		}
	}
	saveState(tasks)
}
