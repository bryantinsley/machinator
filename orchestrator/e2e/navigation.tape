# navigation.tape
Output docs/ui-history/navigation.gif

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
Type "cp -r testdata/vhs-fixture/. $HOME/"
Enter
Type "export PATH=$HOME/bin:$PATH"
Enter
Show

# Start the application (pre-built linux binary for docker)
Type "./setup-tui-linux"
Enter

# Wait for application to start
Sleep 2s

# Navigate to a project
Down
Sleep 500ms
Enter
Sleep 1s

# Navigate in project detail
Down
Sleep 500ms
Down
Sleep 500ms
# Go back to main
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Enter
Sleep 1s

# Navigate to Manage Accounts
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Enter
Sleep 1s

# Go back from accounts
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
Type "y"
Sleep 1s
