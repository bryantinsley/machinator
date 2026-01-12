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
