Output docs/ui-history/orchestrator_run.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 800
Set Padding 20
Set WindowBar Colorful

# Setup environment
Hide
Type "export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PWD"
Enter
Type "cp mock-bd.sh bd && chmod +x bd"
Enter
Type "cp dummy-gemini-linux gemini && chmod +x gemini"
Enter
Type "chmod +x machinator-linux"
Enter
Type "mkdir -p demo-repo/.beads"
Enter
Type "mkdir -p demo-repo/machinator/logs"
Enter
Type "mkdir -p demo-repo/templates"
Enter
Type "cp templates/directive_template.txt demo-repo/templates/"
Enter
Type "cp AGENTS.md demo-repo/"
Enter
Type "mkdir -p demo-repo/bootstrap"
Enter
Type "cp templates/check_quota.sh demo-repo/machinator/"
Enter
Type "export GEMINI_MODE=HAPPY"
Enter
Type "export BD_AGENT_NAME=Gemini-01"
Enter
Type "export MACHINATOR_DIR=$PWD"
Enter
Type "cd demo-repo"
Enter
Type "clear"
Enter
Show

# Start machinator
Type "../machinator-linux"
Enter

# Wait for it to load tasks
Sleep 5s

# Execute a task
Type "e"
Sleep 5s

# Show activity for a while
Sleep 15s

# Quit
Type "q"
Sleep 1s
Type "y"
Sleep 2s
