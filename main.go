package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/Eyob49/gator/internal/config"
	"github.com/Eyob49/gator/internal/database"
	"github.com/lib/pq"
)

type state struct {
	db     *database.Queries
	config *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

func (c *commands) register(name string, handler func(*state, command) error) {
	c.handlers[name] = handler
}

func (c *commands) run(state *state, cmd command) error {
	if handler, ok := c.handlers[cmd.name]; ok {
		return handler(state, cmd)
	}
	return fmt.Errorf("unknown command: %s", cmd.name)
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("username is required")
	}

	username := cmd.args[0]
	user, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user %s does not exist", username)
		}
		return fmt.Errorf("failed to get user: %v", err)
	}
	err = s.config.SetUser(user.Name)
	if err != nil {
		return fmt.Errorf("failed to set user: %v", err)
	}

	fmt.Printf("User %s logged in successfully\n", username)
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("username is required")
	}

	id := uuid.New()

	created_at := time.Now()
	updated_at := time.Now()

	user, err := s.db.CreateUser(context.Background(), database.CreateUserParams{
		ID:        id,
		CreatedAt: created_at,
		UpdatedAt: updated_at,
		Name:      cmd.args[0],
	})
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("user %s already exists", cmd.args[0])
		}
		return fmt.Errorf("failed to create user: %v", err)
	}

	err = s.config.SetUser(user.Name)
	if err != nil {
		return fmt.Errorf("failed to set current user: %v", err)
	}

	fmt.Printf("User %s registered successfully\n", user.Name)
	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.DeleteAllUsers(context.Background())
	if err != nil {
		return fmt.Errorf("failed to delete all users: %v", err)
	}
	fmt.Println("Users deleted successfully")
	return nil
}

func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get users: %v", err)
	}
	for _, user := range users {
		if user.Name == s.config.Username {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}
	return nil
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}

	dbQueries := database.New(db)
	s := &state{
		db:     dbQueries,
		config: cfg,
	}

	cmds := &commands{handlers: make(map[string]func(*state, command) error)}
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	if len(os.Args) < 2 {
		log.Fatalf("No command provided")
	}

	cmd := command{
		name: os.Args[1],
		args: os.Args[2:],
	}

	err = cmds.run(s, cmd)
	if err != nil {
		log.Fatalf("Error executing command: %v", err)
	}
}
