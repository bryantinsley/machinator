# crud.tape
# CI Trigger: 1
Output docs/ui-history/crud.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 800
Set Padding 20
Set WindowBar Colorful

# Setup fake environment
Hide
Type "export HOME=$(mktemp -d)"
Enter
Type "mkdir -p $HOME/.machinator/projects/1"
Enter
Type 'echo "{\"id\": 1, \"name\": \"Demo Project\", \"repo_url\": \"https://github.com/demo/repo.git\", \"agent_count\": 3}" > $HOME/.machinator/projects/1/project.json'
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
Sleep 2s

# Select the project
Enter
Sleep 500ms

# Edit Name
Enter
Sleep 500ms
Type " - Updated"
Sleep 500ms
# Cancel changes
Escape
Sleep 500ms

# Edit Agents
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Enter
Sleep 500ms
# Increase agents
Type "+"
Sleep 200ms
Type "+"
Sleep 200ms
# Cancel (Tab to Apply, Tab to Cancel)
Tab
Sleep 200ms
Tab
Sleep 200ms
Enter
Sleep 500ms

# Attempt Delete
Type "j"
Sleep 200ms
Enter
Sleep 500ms
# Dialog opens. Cancel with 'n'
Type "n"
Sleep 500ms

# Return to Main Menu
Type "j"
Sleep 200ms
Enter
Sleep 500ms

# Quit
Type "q"
Sleep 200ms
Enter
Sleep 500ms