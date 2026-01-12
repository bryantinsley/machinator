#!/bin/bash
case "$1" in
  list)
    echo '[{"id":"bead-1","title":"Task 1","status":"open"},{"id":"bead-2","title":"Task 2","status":"blocked"}]'
    ;;
  ready)
    echo '[{"id":"bead-1","title":"Task 1","status":"open"}]'
    ;;
  show)
    echo "Task 1: Implement basic UI"
    echo "Description: We need a basic UI for the orchestrator."
    ;;
  update)
    # Just succeed silently
    exit 0
    ;;
  close)
    exit 0
    ;;
  init)
    exit 0
  ;;
  create)
    exit 0
  ;;
  *)
    echo "Unknown command $1"
    exit 1
    ;;
esac
