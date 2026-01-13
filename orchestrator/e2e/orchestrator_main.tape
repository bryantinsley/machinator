Output docs/ui-history/orchestrator_main.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 800
Set Padding 20
Set WindowBar Colorful

# Setup environment
Hide
Type "export PATH=$PWD:$PATH"
Enter
Type "cp mock-bd.sh bd && chmod +x bd"
Enter
Type "cp dummy-gemini-linux gemini && chmod +x gemini"
Enter
Type "chmod +x machinator-linux"
Enter
Type "export MACHINATOR_DIR=$PWD"
Enter
Type "mkdir -p .machinator/projects"
Enter
Type "cp testdata/vhs-fixture/.machinator/settings.json .machinator/"
Enter
Type "mkdir -p .machinator/projects/1"
Enter
Type "cp testdata/vhs-fixture/.machinator/projects/1/project.json .machinator/projects/1/"
Enter
Type "export GEMINI_MODE=AUTO_CLOSE"
Enter
Type "cd testdata/fixture-repo"
Enter
Type "clear"
Enter
Show

# Start machinator
Type "../../machinator-linux"
Enter

# Wait for it to load tasks
Sleep 8s

# Press 's' to start orchestration
Type "s"
Sleep 2s

# Watch events appear in activity
Sleep 15s

# Press 'p' to pause
Type "p"
Sleep 5s

# Press 'q', 'y' to quit
Type "q"
Sleep 1s
Type "y"
Sleep 2s