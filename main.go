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
  "github.com/alecthomas/chroma/formatters"
  "github.com/alecthomas/chroma/lexers"
  "github.com/alecthomas/chroma/styles"
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
// on a temporary file with a .md extension (for syntax highlighting) and returns its contents.
func openEditor(initialContent string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Create a temporary file with .md extension for Markdown highlighting
	tmpFile, err := ioutil.TempFile("", "bild_edit_*.md")
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

// editEntireProject using Markdown format
// TODO: Maybe just cut my losses and keep it in the JSON format - I'm just a slut for some syntax highlighting
func editEntireProject(projectName string, config *Config) error {
	// Get or create the project configuration
	proj, exists := config.Projects[projectName]
	if !exists {
		proj = ProjectConfig{Phases: []Phase{}}
	}

	// Build the initial content in Markdown format
	var initialContent strings.Builder
	
	// Project header
	initialContent.WriteString("# Project: " + projectName + "\n\n")
	
	// Instructions
	initialContent.WriteString("Edit commands for each phase below. Instructions:\n")
	initialContent.WriteString("- Order of phases here determines execution order\n")
	initialContent.WriteString("- Commands must be inside ``` blocks\n")
	initialContent.WriteString("- Each phase must be a level 2 heading (##)\n\n")

	// Add existing phases
	for _, phase := range proj.Phases {
		initialContent.WriteString("## " + phase.Name + "\n\n")
		initialContent.WriteString("```bash\n")
		for i, cmd := range phase.Commands {
			initialContent.WriteString(cmd)
			if i < len(phase.Commands)-1 {
				initialContent.WriteString("\n")
			}
		}
		initialContent.WriteString("\n```\n\n")
	}

	// Open editor
	editedContent, err := openEditor(initialContent.String())
	if err != nil {
		return err
	}

	// Parse the edited content
	var newPhases []Phase
	var currentPhase *Phase
	var inCodeBlock bool
	var codeLines []string

	lines := strings.Split(editedContent, "\n")
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and the project header
		if trimmed == "" || strings.HasPrefix(trimmed, "# Project:") || 
		   strings.HasPrefix(trimmed, "Edit commands") || strings.HasPrefix(trimmed, "-") {
			continue
		}

		// Check for phase headers (##)
		if strings.HasPrefix(trimmed, "## ") {
			// If we were building a phase, finalize it
			if currentPhase != nil && len(codeLines) > 0 {
				currentPhase.Commands = codeLines
				newPhases = append(newPhases, *currentPhase)
			}
			
			// Start a new phase
			phaseName := strings.TrimSpace(trimmed[3:])
			currentPhase = &Phase{
				Name:     phaseName,
				Commands: []string{},
			}
			codeLines = nil
			inCodeBlock = false
			continue
		}

		// Handle code blocks
		if trimmed == "```" || trimmed == "```bash" {
			inCodeBlock = !inCodeBlock
			continue
		}

		// Collect commands inside code blocks
		if inCodeBlock && currentPhase != nil && trimmed != "" {
			codeLines = append(codeLines, trimmed)
		}
	}

	// Add the last phase if it exists
	if currentPhase != nil && len(codeLines) > 0 {
		currentPhase.Commands = codeLines
		newPhases = append(newPhases, *currentPhase)
	}

	// Update the project with the new phases
	proj.Phases = newPhases
	config.Projects[projectName] = proj

	// Save the configuration
	if err := saveConfig(config); err != nil {
		return fmt.Errorf("error saving config: %v", err)
	}

	fmt.Printf("Project %s updated with %d phase(s).\n", projectName, len(newPhases))
	for _, phase := range newPhases {
		fmt.Printf("  Phase %s: %d command(s)\n", phase.Name, len(phase.Commands))
	}
	
	return nil
}

// dumpProjectConfig dumps a project's configuration to the local .bild.json file
func dumpProjectConfig(projectName string, config *Config) error {
	// Verify project exists
	proj, exists := config.Projects[projectName]
	if !exists {
		return fmt.Errorf("project %s not found", projectName)
	}

	// Get git repository root
	gitCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := gitCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get git repository root: %v", err)
	}
	repoRoot := strings.TrimSpace(string(output))

	// Create local config with just this project
	localConfig := map[string]ProjectConfig{
		projectName: proj,
	}

	// Marshal the config with proper indentation
	data, err := json.MarshalIndent(localConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write to file
	localConfigPath := filepath.Join(repoRoot, ".bild.json")
	if err := os.WriteFile(localConfigPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	fmt.Printf("Successfully dumped configuration for project '%s' to %s\n", projectName, localConfigPath)
	return nil
}

// highlightCommand returns a syntax-highlighted version of the command
func highlightCommand(command string) string {
    lexer := lexers.Get("bash")
    if lexer == nil {
        lexer = lexers.Fallback
    }
    style := styles.Get("monokai")
    if style == nil {
        style = styles.Fallback
    }
    formatter := formatters.Get("terminal")
    if formatter == nil {
        formatter = formatters.Fallback
    }

    iterator, err := lexer.Tokenise(nil, command)
    if err != nil {
        return command // Return original if highlighting fails
    }

    var buf strings.Builder
    err = formatter.Format(&buf, style, iterator)
    if err != nil {
        return command // Return original if formatting fails
    }

    return buf.String()
}


// loadLocalConfig attempts to load a .bild.json file from the current directory
func loadLocalConfig() (*Config, bool, error) {
    // Check if .bild.json exists in current directory
    if _, err := os.Stat(".bild.json"); os.IsNotExist(err) {
        return nil, false, nil
    }

    data, err := ioutil.ReadFile(".bild.json")
    if err != nil {
        return nil, false, fmt.Errorf("failed to read local config: %v", err)
    }

    var config Config
    if err := json.Unmarshal(data, &config.Projects); err != nil {
        return nil, false, fmt.Errorf("failed to parse local config: %v", err)
    }

    return &config, true, nil
}

// runProject executes the commands for a project.
// If phaseName is empty, all phases are run in order.
// Otherwise, only the specified phase is executed.
func runProject(projectName string, phaseName string, config *Config) error {
    // Always attempt to change to the git repository root
    gitCmd := exec.Command("git", "rev-parse", "--show-toplevel")
    gitOutput, err := gitCmd.Output()
    if err == nil {
        repoRoot := strings.TrimSpace(string(gitOutput))
        fmt.Printf("Changing working directory to repository root: %s\n", repoRoot)
        if err := os.Chdir(repoRoot); err != nil {
            os.Exit(1)  // Exit directly on directory change failure
        }
    } else {
        fmt.Println("Not a git repository; running in current directory.")
    }

    // Try to load local config first
    localConfig, hasLocal, err := loadLocalConfig()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)  // Exit directly on config load failure
    }

    var proj ProjectConfig
    if hasLocal {
        // For local config, just take the first project regardless of name
        for _, p := range localConfig.Projects {
            proj = p
            break
        }
    } else {
        // Fall back to global config
        if projectName == "" {
            fmt.Fprintf(os.Stderr, "Error: project name required when no local config exists\n")
            os.Exit(1)
        }
        var exists bool
        proj, exists = config.Projects[projectName]
        if !exists {
            fmt.Fprintf(os.Stderr, "Error: project %s not found\n", projectName)
            os.Exit(1)
        }
    }

    // If no phase is specified, run all phases
    if phaseName == "" {
        for _, ph := range proj.Phases {
            fmt.Printf("\n📦 Running phase: %s\n", ph.Name)
            
            // Create a shell script that combines all commands in the phase
            var script strings.Builder
            script.WriteString("set -e\n") // Exit on any error
            
            // Add each command to the script
            for _, cmd := range ph.Commands {
                script.WriteString(cmd + "\n")
                // Show the command that will be executed
                highlighted := highlightCommand(cmd)
                fmt.Printf("$ %s\n", highlighted)
            }
            
            // Execute all commands in a single shell process
            cmd := exec.Command("sh", "-c", script.String())
            cmd.Stdout = os.Stdout
            cmd.Stderr = os.Stderr
            cmd.Stdin = os.Stdin
            
            if err := cmd.Run(); err != nil {
                fmt.Fprintf(os.Stderr, "Error: phase %s failed: %v\n", ph.Name, err)
                os.Exit(1)  // Exit directly on command failure
            }
        }
        return nil
    }

    // Run specific phase
    var found bool
    for _, ph := range proj.Phases {
        if ph.Name == phaseName {
            found = true
            fmt.Printf("\n📦 Running phase: %s\n", phaseName)
            
            // Create a shell script for the specific phase
            var script strings.Builder
            script.WriteString("set -e\n")
            
            for _, cmd := range ph.Commands {
                script.WriteString(cmd + "\n")
                highlighted := highlightCommand(cmd)
                fmt.Printf("$ %s\n", highlighted)
            }
            
            cmd := exec.Command("sh", "-c", script.String())
            cmd.Stdout = os.Stdout
            cmd.Stderr = os.Stderr
            cmd.Stdin = os.Stdin
            
            if err := cmd.Run(); err != nil {
                fmt.Fprintf(os.Stderr, "Error: phase %s failed: %v\n", phaseName, err)
                os.Exit(1)  // Exit directly on command failure
            }
            break
        }
    }
    if !found {
        fmt.Fprintf(os.Stderr, "Error: phase %s not found\n", phaseName)
        os.Exit(1)
    }
    return nil
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
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
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
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("Project %s, phase %s updated with %d command(s).\n", projectName, phaseName, len(newCommands))
    return nil
}
// Modify the editCmd to handle both full project and single phase editing
// If no phase is provided, it defaults to the "build" phase.
var editCmd = &cobra.Command{
	Use:   "edit [project] [phase]",
	Short: "Edit build commands for a project",
	Long: `Opens your preferred editor to modify build commands.
If only a project name is provided, allows editing and reordering all phases.
If both project and phase are provided, edits only that specific phase.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		config, err := loadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %v", err)
		}

		if len(args) == 1 {
			return editEntireProject(projectName, config)
		}

		// Edit specific phase (existing behavior)
		phaseName := args[1]
		return editProjectPhase(projectName, phaseName, config)
	},
}

// Modify the listProjects function to use highlighting
func listProjects(config *Config) {
    if len(config.Projects) == 0 {
        fmt.Println("No projects registered.")
        return
    }
    
    fmt.Println("📋 Registered projects:")
    for projName, projConfig := range config.Projects {
        fmt.Printf("\n🔷 Project: %s\n", projName)
        if len(projConfig.Phases) == 0 {
            fmt.Println("  No phases defined.")
        } else {
            for _, ph := range projConfig.Phases {
                fmt.Printf("  📎 Phase: %s (%d command%s)\n", 
                    ph.Name, 
                    len(ph.Commands), 
                    map[bool]string{true: "", false: "s"}[len(ph.Commands) == 1],
                )
                
                // Show highlighted commands
                for _, cmd := range ph.Commands {
                    highlighted := highlightCommand(cmd)
                    fmt.Printf("      $ %s\n", highlighted)
                }
            }
        }
    }
}

// dumpCmd dumps a project's configuration to .bild.json in the git repository root
var dumpCmd = &cobra.Command{
	Use:   "dump [project]",
	Short: "Dump a project's configuration to local .bild.json",
	Long:  "Exports a project's configuration to .bild.json in the git repository root",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		config, err := loadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %v", err)
		}
		return dumpProjectConfig(projectName, config)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Path to configuration file (default: ~/.config/bild/bild.json)")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(dumpCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
