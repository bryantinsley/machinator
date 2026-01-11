# crud.tape
Output docs/ui-history/crud.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 800
Set Padding 20
Set WindowBar Colorful
Set TypingSpeed 100ms

# Setup fake environment
Hide
Type "export HOME=$(mktemp -d)"
Enter
Type "mkdir -p $HOME/.machinator/projects/1"
Enter
Type "echo '{\"id\": 1, \"name\": \"Demo Project\", \"repo_url\": \"https://github.com/demo/repo.git\", \"agent_count\": 3}' > $HOME/.machinator/projects/1/project.json"
Enter
# Create a fake beads directory so it looks nice
Type "mkdir -p $HOME/.machinator/projects/1/agents/1/.beads"
Enter
# Mock bd command to avoid errors
Type "mkdir -p $HOME/bin"
Enter
Type "echo '#!/bin/bash' > $HOME/bin/bd"
Enter
Type "echo 'echo []' >> $HOME/bin/bd"
Enter
Type "chmod +x $HOME/bin/bd"
Enter
Type "export PATH=$HOME/bin:$PATH"
Enter
# Mock gemini CLI to ensure healthy state and predictable cursor (starts at 1)
Type "echo '#!/bin/bash' > $HOME/.machinator/gemini"
Enter
Type "echo 'echo 1.0.0' >> $HOME/.machinator/gemini"
Enter
Type "chmod +x $HOME/.machinator/gemini"
Enter
Show

# Start the application (pre-built linux binary for docker)
Type "./setup-tui-linux"
Enter

# Wait for application to start and load projects
Sleep 5s

# Select the project
# Cursor starts at 1 (Project #1) because Gemini CLI is installed
Enter
Sleep 1s

# We are now in Project Detail View
# Cursor is at Name (0)
# 1. Edit Name
Enter
Sleep 1s
Type " - Updated"
Sleep 1s
# Cancel changes
Escape
Sleep 1s

# 2. Edit Agents
# Move down to Agents (Name -> Repo -> Agents)
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Enter
Sleep 1s
# Increase agents
Type "+"
Sleep 500ms
Type "+"
Sleep 500ms
# Cancel (Tab to Apply, Tab to Cancel)
Tab
Sleep 500ms
Tab
Sleep 500ms
Enter
Sleep 1s

# 3. Attempt Delete
# Move down to Delete (Agents -> Delete)
Type "j"
Sleep 500ms
Enter
Sleep 1s
# Dialog opens. Cancel with 'n'
Type "n"
Sleep 1s

# 4. Return to Main Menu
# Move down to Back (Delete -> Back)
Type "j"
Sleep 500ms
Enter
Sleep 1s

# Quit
Type "q"
Sleep 500ms
Enter
Sleep 1s