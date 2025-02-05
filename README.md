# 🛠️ bild 🛠️

## Overview

**bild** is a command-line tool that helps manage and execute build commands for projects. It allows users to define **explicit build phases** such as `configure`, `build`, and `test`, and run them in a structured manner.

The tool supports:

- ✅ **Automatic project detection** (via Git repository name - Will probably break)
- ✅ **Explicit build phases** (configure, build, test, etc.)
- ✅ **Execution of all phases in order** (or a specific phase if needed - order: configure -> build -> test)
- ✅ **Easy command editing** using the `$EDITOR` environment variable
- ✅ **Configuration persistence** in `~/.config/bild/bild.json`
- ✅ **Local project configuration** via `.bild.json` in repository root
- ✅ **Syntax highlighted commands** cause looking at plain text is for weenies

---

## Installation

1. **Clone the repository**

   ```sh
   git clone https://github.com/rkabrick/bild.git
   cd bild
   ```

2. **Initialize the module and fetch dependencies**

   ```sh
   go mod init bild
   go get github.com/spf13/cobra
   ```

3. **Build the binary**

   ```sh
   go build -o bild main.go
   ```

4. **Add the binary to a directory in your `PATH`** (optional)

   ```sh
    ln -s $(realpath ./bild) ~/.local/bin/bild
   ```

---

## Configuration

**bild** supports two levels of configuration:

1. **Global configuration** (default: `~/.config/bild/bild.json`)
2. **Local repository configuration** (`.bild.json` in repository root)

### Global Configuration

The global configuration file contains all registered projects and their build phases.

### Local Configuration

You can dump a project's configuration to a local `.bild.json` file in your repository root, making it portable and version-controllable. Also (more importantly) you can just run `bild` and it will run all the phases for you based on that configuration.

### Example Configuration

```json
{
  "projects": {
    "my_project": {
      "phases": [
        {
          "name": "configure",
          "commands": [
            "cmake ../ -DCMAKE_EXPORT_BUILD_COMMANDS=On -DCMAKE_BUILD_TYPE=RelWithDebInfo -GNinja"
          ]
        },
        {
          "name": "build",
          "commands": ["ninja -j4"]
        },
        {
          "name": "test",
          "commands": ["ctest"]
        }
      ]
    }
  }
}
```

You can override the global config location using:

```sh
bild --config /path/to/custom_config.json
```

---

## Usage

### 1. Running Build Commands

- **Run all phases** (autodetect project from Git repo):

  ```sh
  bild run
  ```

  This runs **all phases** (e.g., `configure → build → test`) in order.

- **Run a specific phase** (e.g., only `build`):

  ```sh
  bild run my_project build
  ```

- **Run all phases for a specific project**:

  ```sh
  bild run my_project
  ```

### 2. Editing Build Commands

- **Edit all phases for a project**:

  ```sh
  bild edit my_project
  ```

  This opens a temporary `.md` file in `$EDITOR` with full Markdown support. You'll see something like:

  ````markdown
  # Project: my_project

  Edit commands for each phase below. Instructions:

  - Order of phases here determines execution order
  - Commands must be inside ``` blocks
  - Each phase must be a level 2 heading (##)

  ## configure

  ```bash
  cmake ../ -DCMAKE_EXPORT_BUILD_COMMANDS=On -DCMAKE_BUILD_TYPE=RelWithDebInfo -GNinja
  ```
  ````

  ## build

  ```bash
  ninja -j4
  ```

  ```

  The order of phases in this file determines their execution order. Move them around to change the sequence!

  ```

- **Edit a specific phase**:

  ```sh
  bild edit my_project configure
  ```

### 3. Listing Projects & Phases

- **View all registered projects and their phases**:

  ```sh
  bild list
  ```

  Example output:

  ```
  Project: my_project
    Phase: configure (1 command)
    Phase: build (1 command)
    Phase: test (1 command)
  ```

### 4. Managing Project Configuration

- **Set a custom configuration file**:

  ```sh
  bild --config /path/to/my_config.json edit my_project
  ```

- **Dump project configuration to repository**:

  ```sh
  bild dump my_project
  ```

  This creates a `.bild.json` file in your repository root containing the project's configuration, making it:

  - 📦 Portable (share with your team)
  - 🔒 Version-controllable (track changes)
  - 🚀 Easy to set up (clone and go)

---

## Examples

### Basic C++ Project Setup

```sh
bild edit my_cpp_project configure
```

_Edit configure phase (example commands)_:

```sh
cmake -B build -DCMAKE_BUILD_TYPE=Release
```

```sh
bild edit my_cpp_project build
```

_Edit build phase (example commands)_:

```sh
cd build
make -j$(nproc)
```

```sh
bild edit my_cpp_project test
```

_Edit test phase (example commands)_:

```sh
cd build
ctest --output-on-failure
```

**Run the entire process**:

```sh
bild run
```

This will run:
1️⃣ `configure` → 2️⃣ `build` → 3️⃣ `test`

**Run only the test phase**:

```sh
bild run my_cpp_project test
```

### Share Project Configuration

To share your project configuration with your team:

1. **Dump the configuration**:

   ```sh
   bild dump my_cpp_project
   ```

2. **Commit the `.bild.json` file**:

   ```sh
   git add .bild.json
   git commit -m "Add bild configuration"
   ```

Now your teammates can clone the repo and use `bild` immediately! (So long as they use my stupid tool too!)

---

## Features

✅ **Automatic Git Repository Detection**

- If run inside a Git project, `bild` automatically infers the project name.

✅ **Explicit Build Phases**

- Define `configure`, `build`, `test`, or any custom phase.

✅ **Structured Execution Order**

- Runs phases in sequence unless overridden.

✅ **Persistent Configuration**

- Global config in `~/.config/bild/bild.json`
- Local config in repository's `.bild.json`

✅ **Easy Editing via `$EDITOR`**

- Uses a `.sh` file for syntax highlighting in whatever editor you use.

✅ **Portable & Lightweight**

- Quite literally a digital feather
- Now with shareable project configs cause README's we're just too damn hard for this procrastinator 🎉

---

## Future Enhancements (TODO)

- None. It's perfect. 🌟

- JK... I'm just not creative enough to think of any.

---

## Troubleshooting

### **Q: How do I check what commands are registered?**

🔹 Use `bild list` to view all projects & their phases.

### **Q: Can I edit commands in VS Code instead of Vim (Cause I'm a weenie)?**

🔹 Set your `$EDITOR` environment variable:

```sh
export EDITOR="code -w"
```

---

## Contributing

🔧 **Contributions Welcome!**

1. Fork the repository.
2. Create a new feature branch.
3. Submit a pull request.

---

## License

**bild** is licensed under the **MIT License**.  
Feel free to modify and distribute.

---

## Author

🔹 Maintained by **Me... poorly**  
📧 Contact: `echo "<your grievance>" >> /dev/null`

🔗 GitHub: [rkabrick/bild](https://github.com/rkabrick/bild)
