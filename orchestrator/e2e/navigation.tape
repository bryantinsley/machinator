# navigation.tape
Output docs/ui-history/navigation.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 800
Set Padding 20
Set WindowBar Colorful

# Start the application
Type "go run orchestrator/cmd/setup/main.go"
Enter

# Wait for application to start
Sleep 5s

# Navigate down
Down
Sleep 1s
Down
Sleep 1s

# Navigate up
Up
Sleep 1s
Up
Sleep 1s

# Quit
Type "q"
Sleep 1s
Enter
Sleep 1s
