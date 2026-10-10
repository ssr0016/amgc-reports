# Customization Guide

How to customize this starter kit.

## 1. Rename Project

Update go.mod:
    sed -i 's|github.com/ssr0016/template|github.com/YOUR_USERNAME/YOUR_PROJECT|g' go.mod

Update all Go imports:
    grep -rl "github.com/ssr0016/template" --include="*.go" . | while read f; do
        sed -i 's|github.com/ssr0016/template|github.com/YOUR_USERNAME/YOUR_PROJECT|g' "$f"
    done
    go mod tidy

Also update:
- README.md (repo URL)
- docker-compose.yaml (container names)
- .github/workflows/ci.yml (Codecov)

## 2. Add a New Module

### Step 1: Create migration
    make migrate-create NAME=create_posts

Edit the generated SQL file and run:
    make migrate-up

### Step 2: Create model
Create internal/model/post.go with your struct.

### Step 3: Create repository
Create internal/repository/post_repo.go using Squirrel queries.

### Step 4: Create handler
Create internal/handler/post_handler.go.

### Step 5: Register routes
Update internal/router/router.go.

### Step 6: Wire in main.go
    postRepo := repository.NewPostRepo(db)
    postHandler := handler.NewPostHandler(postRepo)
    router.Setup(e, sm, authHandler, userHandler, postHandler)

## 3. Add Middleware

Create internal/middleware/yourname.go with echo.MiddlewareFunc.

Register in main.go:
    e.Use(ourmiddleware.YourMiddleware())

## 4. Change Database

From Postgres to MySQL:
- Replace pgx with go-sql-driver/mysql
- Update internal/database/db.go
- Use sq.Question placeholder
- Update migrations

From Postgres to SQLite:
- Use modernc.org/sqlite
- Use sq.Question placeholder
- Update migrations

## 5. Add Environment Variable

Step 1: Add to struct in internal/config/config.go
    NewVar string

Step 2: Load in Load() function
    NewVar: getEnv("NEW_VAR", "default"),

Step 3: Add to .env
    NEW_VAR=my-value

## 6. Common Patterns

### Error handling
    return apperror.NotFound("not found")
    return apperror.Internal("failed").WithError(err)
    return apperror.Validation("invalid input")

### Logging
    log.Info("event", "key", "value")
    log.Error("failed", "error", err)

### Validation
    type Request struct {
        Email string `json:"email" validate:"required,email"`
        Name  string `json:"name" validate:"required,min=2"`
    }

### Squirrel query
    query, args, err := r.db.Builder.
        Select("id", "name").
        From("users").
        Where(sq.Eq{"id": id}).
        ToSql()

## 7. File Naming

- Model: model/{name}.go
- Repository: repository/{name}_repo.go
- Service: service/{name}_service.go
- Handler: handler/{name}_handler.go
- Middleware: middleware/{name}.go
- Test: {name}_test.go

## 8. Remove Features

Remove PgAdmin:
- Delete pg-admin service from docker-compose.yaml
- Remove PGADMIN_* from .env

Remove Swagger:
- Remove echoSwagger import from main.go
- Remove e.GET("/swagger/*", ...) route
- Delete docs/ folder

Remove Rate Limiting:
- Remove authLimit from router.go
- Delete internal/middleware/ratelimit.go

## 9. Testing Checklist

Before commit:
    make test    # All tests pass
    make lint    # No lint issues
    make build   # Builds OK

Before deploy:
    docker build -t my-app .
    docker run -p 9099:8080 my-app
    curl http://localhost:9099/health

## 10. Deploy Checklist

- [ ] Update go.mod module name
- [ ] Update all imports
- [ ] Update .env for production
- [ ] Generate strong SESSION_SECRET
- [ ] Set APP_ENV=production
- [ ] Set LOG_FORMAT=json
- [ ] Configure CORS_ALLOWED_ORIGINS
- [ ] Run make test (all pass)
- [ ] Run make lint (no issues)
- [ ] Build Docker image
- [ ] Test container locally
- [ ] Deploy to production
- [ ] Verify health endpoint
- [ ] Check logs for errors
