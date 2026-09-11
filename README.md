# Gator - RSS Feed Aggregator

Gator is a CLI tool for fetching, storing, and browsing RSS feeds in your terminal.

## Prerequisites

- **Go** 1.21+ ([install](https://golang.org/doc/install))
- **PostgreSQL** 13+ ([install](https://www.postgresql.org/download/))

## Installation

```bash
go install github.com/Eyob49/gator@latest
```

This compiles Gator into a static binary and places it in your `$GOPATH/bin`.

## Setup

### 1. Create the database

```bash
psql -U postgres
CREATE DATABASE gator;
\q
```

### 2. Run migrations

```bash
cd sql/schema
goose postgres "postgres://postgres:YOUR-PASSWORD@localhost:5432/gator?sslmode=disable" up
cd ../..
```

### 3. Configure Gator

Gator stores config in `~/.gatorconfig.json`. Run any command to auto-create it:

```bash
gator register alice
```

## Usage

### Register & Login

```bash
gator register username
gator login username
```

### Manage Feeds

```bash
gator addfeed "My Blog" "https://example.com/feed.xml"
gator feeds          # List all feeds
gator follow "https://example.com/feed.xml"
gator unfollow "https://example.com/feed.xml"
gator following      # List feeds you follow
```

### Aggregate & Browse

```bash
gator agg 30s        # Fetch feeds every 30 seconds (runs forever)
gator browse         # Show 2 latest posts
gator browse 10      # Show 10 latest posts
```

## Architecture

- **PostgreSQL** stores users, feeds, posts, and follow relationships
- **Goose** manages database migrations
- **SQLC** generates type-safe database code
- **CLI Registry pattern** for command dispatching
- **Middleware** for login authentication

## Development

```bash
git clone https://github.com/Eyob49/gator.git
cd gator
go mod download
go run . register alice
go run . agg 10s
```

## License

MIT