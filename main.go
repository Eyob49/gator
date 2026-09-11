package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("User-Agent", "GatorRSS/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	var feed RSSFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feed: %v", err)
	}
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}

	return &feed, nil
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

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("feed name and URL are required")
	}

	feedID := uuid.New()
	feedFollowID := uuid.New()
	created_at := time.Now()
	updated_at := time.Now()

	feed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        feedID,
		CreatedAt: created_at,
		UpdatedAt: updated_at,
		Name:      cmd.args[0],
		Url:       cmd.args[1],
		UserID:    user.ID,
	})
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("url %s already exists", cmd.args[1])
		}
		return fmt.Errorf("failed to create feed: %v", err)
	}

	_, err = s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        feedFollowID,
		CreatedAt: created_at,
		UpdatedAt: updated_at,
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("already following this feed")
		}
		return fmt.Errorf("failed to create feed: %v", err)
	}

	fmt.Printf("ID: %s\nCreated_at: %s\nUpdated_at: %s\nName: %s\nUrl: %s\nUser_ID: %s", feed.ID, feed.CreatedAt, feed.UpdatedAt, feed.Name, feed.Url, feed.UserID)

	return nil
}

func handlerFeeds(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("no extra field needed")
	}

	feeds, err := s.db.GetFeedsWithUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error fetching fields: %v", err)
	}

	for i := range feeds {
		fmt.Printf("Feed Name: %s\nURL: %s\nUser: %s\n---\n", feeds[i].Name, feeds[i].Url, feeds[i].UserName)
	}

	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("URL is required")
	}

	feedURL, err := s.db.GetFeedByURL(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("feed not found: %v", err)
	}

	id := uuid.New()
	created_at := time.Now()
	updated_at := time.Now()

	feedFollow, err := s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        id,
		CreatedAt: created_at,
		UpdatedAt: updated_at,
		UserID:    user.ID,
		FeedID:    feedURL.ID,
	})
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("already following this feed")
		}
		return fmt.Errorf("failed to create feed: %v", err)
	}

	fmt.Printf("Feed name: %s\nUser name: %s\n", feedFollow.FeedName, feedFollow.UserName)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("no extra field needed")
	}

	feedFollowsForUser, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("failed to get user feed follows: %v", err)
	}

	if len(feedFollowsForUser) == 0 {
		fmt.Println("You are not following any feeds yet.")
		return nil
	}

	for i := range feedFollowsForUser {
		fmt.Printf("Feed name: %s\n", feedFollowsForUser[i].FeedName)
	}

	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("feed URL is required")
	}

	feedURL, err := s.db.GetFeedByURL(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("feed not found: %v", err)
	}

	err = s.db.DeleteFeedFollow(context.Background(), database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feedURL.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete feed follow: %w", err)
	}

	fmt.Printf("Unfollowed %s\n", feedURL.Name)

	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	parsedLimit, err := strconv.Atoi(cmd.args[0])
	if err != nil {
		return fmt.Errorf("invalid limit: %v", err)
	}
	limit := int32(parsedLimit)

	postsForUser, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  limit,
	})

	for i := range postsForUser {
		fmt.Printf("Title: %s\nURL: %s\nDescription: %s\nPublished: %v\n\n",
			postsForUser[i].Title,
			postsForUser[i].Url,
			postsForUser[i].Description.String,
			postsForUser[i].PublishedAt.Time)
	}

	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		currentUser, err := s.db.GetUser(context.Background(), s.config.Username)
		if err != nil {
			return fmt.Errorf("failed to get user: %v", err)
		}
		err = handler(s, cmd, currentUser)
		return err
	}
}

func scrapeFeeds(s *state) error {
	nextFeed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No feeds to fetch right now")
			return nil
		}
		return fmt.Errorf("failed to get next feed: %v", err)
	}

	fmt.Printf("Fetching feed: %s\n", nextFeed.Name)

	err = s.db.MarkFeedFetched(context.Background(), nextFeed.ID)
	if err != nil {
		return fmt.Errorf("failed to mark feed fetched: %v", err)
	}

	feed, err := fetchFeed(context.Background(), nextFeed.Url)
	if err != nil {
		return fmt.Errorf("failed to fetch feed: %v", err)
	}

	for _, item := range feed.Channel.Item {
		publishedAt := sql.NullTime{}
		t, err := time.Parse(time.RFC1123Z, item.PubDate)
		if err != nil {
			fmt.Printf("Failed to parse time %s: %v\n", item.PubDate, err)
		} else {
			publishedAt = sql.NullTime{Time: t, Valid: true}
		}

		_, err = s.db.CreatePost(context.Background(), database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       item.Title,
			Url:         item.Link,
			Description: sql.NullString{String: item.Description, Valid: item.Description != ""},
			PublishedAt: publishedAt,
			FeedID:      nextFeed.ID,
		})
		if err != nil {

			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				continue
			}

			fmt.Printf("Failed to save post %s: %v\n", item.Title, err)
		}
	}

	return nil
}

func newCommands() *commands {
	cmds := &commands{handlers: make(map[string]func(*state, command) error)}
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("agg", handlerAgg)
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))
	return cmds
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("time duration is required")
	}

	timeBetweenRequests, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("error parsing the time: %v", err)
	}

	fmt.Printf("Collecting feeds every %v\n", timeBetweenRequests)

	ticker := time.NewTicker(timeBetweenRequests)
	for ; ; <-ticker.C {
		err := scrapeFeeds(s)
		if err != nil {
			fmt.Printf("Error scraping: %v\n", err)
		}
	}
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

	cmds := newCommands()
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
