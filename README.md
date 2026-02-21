# Traffic Simulator 🚦

**High-performance concurrent traffic simulator in Go for load testing personal projects**

## 🎯 Features

- ✅ **Massive Concurrency** - Simulate thousands of concurrent users
- ✅ **Realistic User Behavior** - Configurable user journeys with think times
- ✅ **Multiple Endpoints** - Hit different endpoints with weighted probability
- ✅ **Real-time Statistics** - Live RPS, success rate, response times
- ✅ **Graceful Ramp-up** - Gradually increase load to avoid shocking the system
- ✅ **Error Simulation** - Configurable error rates for realistic testing
- ✅ **Clean Shutdown** - Handle Ctrl+C gracefully

## 🚀 Quick Start

### Build
```bash
cd traffic-simulator
go build -o traffic-sim ./cmd
```

### Basic Usage
```bash
# Test localhost with 100 concurrent users for 1 minute
./traffic-sim -url http://localhost:8080 -users 100 -duration 1m
```

### Advanced Usage
```bash
# Heavy load test: 1000 users, 5 minute duration, 30s ramp-up
./traffic-sim -url http://localhost:8080 -users 1000 -duration 5m -rampup 30s
```

## 📊 Example Output

```
🎯 Traffic Simulator v1.0.0
============================

🚀 Starting traffic simulation...
   Target: http://localhost:8080
   Concurrent Users: 100
   Duration: 1m0s
   Random Seed: 1708441200

✅ All 100 users active

📊 [12:00:05] Users: 100 | Requests: 523 | Success: 98.5% | Avg RT: 145ms | RPS: 104.6
📊 [12:00:10] Users: 100 | Requests: 1047 | Success: 98.2% | Avg RT: 152ms | RPS: 104.7
📊 [12:00:15] Users: 100 | Requests: 1568 | Success: 98.4% | Avg RT: 148ms | RPS: 104.5

⏹️  Simulation duration completed

📊 Final Statistics:
   Total Requests:    6284
   Successful:        6178 (98.3%)
   Failed:            106 (1.7%)
   Avg Response Time: 149ms
   Duration:          1m0s
   Requests/Second:   104.73
```

## ⚙️ Configuration

### Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | "" | Base URL to test (required for real HTTP requests) |
| `-users` | 100 | Number of concurrent users |
| `-duration` | 1m | Test duration (e.g., 30s, 1m, 5m) |
| `-rampup` | 10s | Ramp-up time to reach full concurrency |
| `-config` | "" | Path to JSON configuration file |

### User Actions

Default user actions include:
- **Homepage Visit** - GET /
- **API Health Check** - GET /health
- **Browse Content** - GET /api/items, /api/items/1
- **User Login Flow** - POST /api/login → GET /api/user/profile
- **Heavy Load Search** - GET /api/search?q=test

To customize, modify `getDefaultUserActions()` in `cmd/main.go` or use a config file.

## 🔧 Customization Examples

### Simulate E-commerce Site
```go
[]UserAction{
    {
        Name: "Product Browse",
        Endpoints: []Endpoint{
            {Method: "GET", Path: "/products", MinDelayMs: 100, MaxDelayMs: 500},
            {Method: "GET", Path: "/products/123", MinDelayMs: 50, MaxDelayMs: 300},
        },
        ThinkTimeMs: 2000,
    },
    {
        Name: "Add to Cart",
        Endpoints: []Endpoint{
            {Method: "POST", Path: "/cart/add", MinDelayMs: 200, MaxDelayMs: 800},
        },
        ThinkTimeMs: 1000,
    },
}
```

### Simulate API Backend
```go
[]UserAction{
    {
        Name: "Read Heavy",
        Endpoints: []Endpoint{
            {Method: "GET", Path: "/api/users", Weight: 70},
            {Method: "GET", Path: "/api/posts", Weight: 30},
        },
    },
    {
        Name: "Write Operations",
        Endpoints: []Endpoint{
            {Method: "POST", Path: "/api/posts", ErrorRate: 0.05},
        },
    },
}
```

## 📈 Performance Tips

1. **Start Small**: Begin with 10-50 users, then scale up
2. **Monitor Resources**: Watch your server's CPU, memory, and connections
3. **Use Realistic Think Times**: Humans don't request instantly (1-3s typical)
4. **Ramp Up Gradually**: Give your server time to warm up
5. **Test Different Scenarios**: Mix of read/write operations

## 🎯 Use Cases

### Personal Projects
- Test before deploying to production
- Find bottlenecks early
- Validate caching strategies
- Test database connection pooling

### Production Apps
- Load testing before major releases
- Capacity planning
- Identify scaling limits
- Test auto-scaling triggers

### API Development
- Validate rate limiting
- Test error handling under load
- Measure actual vs expected performance

## 🛠️ Building for Production

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o traffic-sim-linux ./cmd

# macOS
GOOS=darwin GOARCH=amd64 go build -o traffic-sim-macos ./cmd

# Windows
GOOS=windows GOARCH=amd64 go build -o traffic-sim-windows.exe ./cmd
```

## 📝 License

MIT License - See LICENSE file

---

**Built with ❤️ for testing personal projects before they go to production**
