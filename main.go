package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Global variable to hold the configuration file path (set via --config flag).
var configFile string

// Phase represents an ordered set of commands for one phase (e.g. "configure", "build", "test").
type Phase struct {
	Name     string   `json:"name"`
	Commands []string `json:"commands"`
}

// ProjectConfig holds the phases for a given project.
type ProjectConfig struct {
	Phases []Phase `json:"phases"`
}

// Config holds a mapping from project names to their configurations.
type Config struct {
	Projects map[string]ProjectConfig `json:"projects"`
}

// getConfigFilePath returns the configuration file path.
// If the --config flag was provided, that value is used (with "~" expanded).
// Otherwise, it defaults to ~/.config/bild/bild.json.
func getConfigFilePath() (string, error) {
	if configFile != "" {
		if strings.HasPrefix(configFile, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(home, configFile[1:]), nil
		}
		return configFile, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".config", "bild")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "bild.json"), nil
}

// loadConfig reads the configuration from file (or returns an empty config if the file doesn't exist).
func loadConfig() (*Config, error) {
	path, err := getConfigFilePath()
	if err != nil {
		return nil, err
	}

	config := &Config{
		Projects: make(map[string]ProjectConfig),
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config, nil
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

// saveConfig writes the configuration to file.
func saveConfig(config *Config) error {
	path, err := getConfigFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0644)
}

// getGitRepoName determines the repository name by running "git rev-parse --show-toplevel"
// and returning the basename of the resulting path.
// TODO: Maybe augment this to use git remote -v to get the actual repo name in case the directory name differs.
//      This would require a more complex parsing of the output... And probably wouldn't work. 
func getGitRepoName() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	repoPath := strings.TrimSpace(string(output))
	return filepath.Base(repoPath), nil
}

// openEditor opens the user's preferred editor (from $EDITOR, defaulting to "vi")
// on a temporary file with a .sh extension (for syntax highlighting) and returns its contents.
func openEditor(initialContent string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Create a temporary file with a .sh extension.
	tmpFile, err := ioutil.TempFile("", "bild_edit_*.sh")
	if err != nil {
		return "", err
	}
	tmpFileName := tmpFile.Name()

	if initialContent != "" {
		if _, err := tmpFile.WriteString(initialContent); err != nil {
			return "", err
		}
	}
	tmpFile.Close()

	cmd := exec.Command(editor, tmpFileName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	content, err := ioutil.ReadFile(tmpFileName)
	if err != nil {
		return "", err
	}
	os.Remove(tmpFileName)
	return string(content), nil
}

// runProject executes the commands for a project.
// If phaseName is empty, all phases are run in order.
// Otherwise, only the specified phase is executed.
func runProject(projectName string, phaseName string, config *Config) error {
	// Always attempt to change to the git repository root.
	gitCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	gitOutput, err := gitCmd.Output()
	if err == nil {
		repoRoot := strings.TrimSpace(string(gitOutput))
		fmt.Printf("Changing working directory to repository root: %s\n", repoRoot)
		if err := os.Chdir(repoRoot); err != nil {
			return fmt.Errorf("failed to change directory to %s: %v", repoRoot, err)
		}
	} else {
		fmt.Println("Not a git repository; running in current directory.")
	}

	proj, ok := config.Projects[projectName]
	if !ok {
		return fmt.Errorf("project %s not found", projectName)
	}

	// If no phase is specified, run all phases.
	if phaseName == "" {
		for _, ph := range proj.Phases {
			fmt.Printf("Running phase: %s\n", ph.Name)
			for _, commandLine := range ph.Commands {
				fmt.Printf("Running: %s\n", commandLine)
				cmd := exec.Command("sh", "-c", commandLine)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("command failed in phase %s: %s, error: %v", ph.Name, commandLine, err)
				}
			}
		}
		return nil
	}

	// Otherwise, run only the specified phase.
	var found bool
	for _, ph := range proj.Phases {
		if ph.Name == phaseName {
			found = true
			fmt.Printf("Running phase: %s\n", phaseName)
			for _, commandLine := range ph.Commands {
				fmt.Printf("Running: %s\n", commandLine)
				cmd := exec.Command("sh", "-c", commandLine)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("command failed in phase %s: %s, error: %v", phaseName, commandLine, err)
				}
			}
			break
		}
	}
	if !found {
		return fmt.Errorf("phase %s not found for project %s", phaseName, projectName)
	}
	return nil
}

// editProjectPhase opens the editor to modify the commands for a given phase of a project.
// If the project or phase does not exist, they are created.
func editProjectPhase(projectName string, phaseName string, config *Config) error {
	// Get or create the project configuration.
	proj, exists := config.Projects[projectName]
	if !exists {
		proj = ProjectConfig{Phases: []Phase{}}
	}
	// Search for the phase.
	var phase *Phase
	for i, ph := range proj.Phases {
		if ph.Name == phaseName {
			phase = &proj.Phases[i]
			break
		}
	}
	if phase == nil {
		// Create a new phase.
		newPhase := Phase{
			Name:     phaseName,
			Commands: []string{},
		}
		proj.Phases = append(proj.Phases, newPhase)
		phase = &proj.Phases[len(proj.Phases)-1]
	}

	// Build the initial content for editing.
	var initialContent string
	if len(phase.Commands) > 0 {
		initialContent = strings.Join(phase.Commands, "\n")
	} else {
		initialContent = "# Enter one command per line for phase '" + phaseName + "'.\n# Lines starting with '#' are ignored.\n"
	}

	editedContent, err := openEditor(initialContent)
	if err != nil {
		return err
	}

	// Parse the edited content.
	var newCommands []string
	for _, line := range strings.Split(editedContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		newCommands = append(newCommands, trimmed)
	}
	phase.Commands = newCommands

	// Update the project configuration.
	config.Projects[projectName] = proj
	if err := saveConfig(config); err != nil {
		return fmt.Errorf("error saving config: %v", err)
	}
	fmt.Printf("Project %s, phase %s updated with %d command(s).\n", projectName, phaseName, len(newCommands))
	return nil
}

// listProjects prints all projects along with their phases and command counts.
func listProjects(config *Config) {
	if len(config.Projects) == 0 {
		fmt.Println("No projects registered.")
		return
	}
	fmt.Println("Registered projects:")
	for projName, projConfig := range config.Projects {
		fmt.Printf("Project: %s\n", projName)
		if len(projConfig.Phases) == 0 {
			fmt.Println("  No phases defined.")
		} else {
			for _, ph := range projConfig.Phases {
				fmt.Printf("  Phase: %s (%d command(s))\n", ph.Name, len(ph.Commands))
			}
		}
	}
}

//
// Cobra commands
//

// rootCmd is the primary command. If no subcommand is provided and no arguments are given,
// it deduces the project from the Git repository and runs all phases.
var rootCmd = &cobra.Command{
	Use:   "bild",
	Short: "Bild is a CLI tool for managing build commands for your projects with explicit phases",
	Long:  "Bild is a CLI tool for registering, editing, and executing build commands organized into explicit phases (e.g. configure, build, test). When no phase is specified, all phases are run.",
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectName string
		if len(args) == 0 {
			var err error
			projectName, err = getGitRepoName()
			if err != nil {
				return fmt.Errorf("could not determine project name from git repository; please provide project name explicitly")
			}
		} else {
			projectName = args[0]
		}
		config, err := loadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %v", err)
		}
		// No phase specified → run all phases.
		return runProject(projectName, "", config)
	},
}

// runCmd executes the build commands for a project. Optionally, a specific phase can be run.
// If no project is provided, it is deduced from the git repository. If no phase is provided,
// all phases are run.
var runCmd = &cobra.Command{
	Use:   "run [project] [phase]",
	Short: "Run build commands for a project (default: run all phases)",
	Long:  "Executes the build commands for the given project. If a phase is specified, only that phase is executed; otherwise, all phases are run in order. If no project is provided, it is deduced from the Git repository.",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectName, phaseName string
		if len(args) == 0 {
			var err error
			projectName, err = getGitRepoName()
			if err != nil {
				return fmt.Errorf("could not determine project name from git repository; please provide project name explicitly")
			}
		} else if len(args) == 1 {
			projectName = args[0]
		} else if len(args) == 2 {
			projectName = args[0]
			phaseName = args[1]
		}
		config, err := loadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %v", err)
		}
		return runProject(projectName, phaseName, config)
	},
}

// editCmd opens the editor to modify the build commands for a specific phase of a project.
// If no phase is provided, it defaults to the "build" phase.
var editCmd = &cobra.Command{
	Use:   "edit [project] [phase]",
	Short: "Edit build commands for a specific phase of a project (default phase: build)",
	Long:  "Opens your preferred editor to modify the build commands for the specified phase of a project. If no phase is provided, the 'build' phase is assumed.",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		phaseName := "build"
		if len(args) == 2 {
			phaseName = args[1]
		}
		config, err := loadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %v", err)
		}
		return editProjectPhase(projectName, phaseName, config)
	},
}

// listCmd displays all projects along with their phases.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects and their build phases",
	Long:  "Displays a list of all projects along with their defined build phases and the number of commands in each phase.",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := loadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %v", err)
		}
		listProjects(config)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Path to configuration file (default: ~/.config/bild/bild.json)")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(listCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
