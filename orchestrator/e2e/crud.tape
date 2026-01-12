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
Type "cp -r testdata/vhs-fixture/. $HOME/"
Enter
Type "export PATH=$HOME/bin:$PATH"
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
# Save changes
Enter
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
# Apply (Tab to Apply)
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