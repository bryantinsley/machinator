Output docs/ui-history/orchestrator_main.gif

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

# Prepare fixture repo
Type "mkdir -p fixture-repo/.beads"
Enter
Type "cp -r testdata/fixture-repo/.beads/. fixture-repo/.beads/"
Enter
Type "cp testdata/fixture-repo/AGENTS.md fixture-repo/"
Enter

# Prepare machinator logs and templates
Type "mkdir -p fixture-repo/machinator/logs"
Enter
Type "mkdir -p fixture-repo/templates"
Enter
Type "cp templates/directive_template.txt fixture-repo/templates/"
Enter
Type "mkdir -p fixture-repo/bootstrap"
Enter
Type "cp bootstrap/check_quota.sh fixture-repo/machinator/"
Enter

Type "export GEMINI_MODE=HAPPY"
Enter
Type "export BD_AGENT_NAME=Gemini-01"
Enter
Type "export MACHINATOR_DIR=$PWD"
Enter
Type "cd fixture-repo"
Enter
Type "clear"
Enter
Show

# Start machinator
Type "../machinator-linux"
Enter

# Wait for it to load tasks
Sleep 8s

# Start orchestration
Type "s"
Sleep 15s

# Pause orchestration
Type "p"
Sleep 5s

# Quit
Type "q"
Sleep 1s
Type "y"
Sleep 2s
