#!/bin/bash

tmux new-session -d -s dev_env -n chirpy

tmux new-window -n server
tmux send-keys "go run main.go" C-m

tmux new-window -n postgres
tmux send-keys "psql postgres://postgres:postgres@localhost:5432/chirpy?sslmode=disable" C-m

