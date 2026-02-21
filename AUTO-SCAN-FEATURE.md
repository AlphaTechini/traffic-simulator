# Auto-Scan Feature - Backend Route Discovery

**Automatically discover and test your backend endpoints with zero configuration!**

---

## 🎯 What It Does

The auto-scan feature:
1. **Detects your backend framework** (Express, Fastify, NestJS, etc.)
2. **Discovers available routes** via multiple methods:
   - OpenAPI/Swagger documentation
   - Common endpoint patterns
   - Intelligent fuzzing
3. **Generates realistic user actions** from discovered routes
4. **Starts load testing immediately** - no manual configuration needed!

---

## 🚀 Quick Start

### **Basic Auto-Scan**
```bash
./traffic-sim -url http://localhost:3000 -scan -users 100 -duration 2m
```

That's it! The simulator will:
- Scan your backend at `http://localhost:3000`
- Discover all available routes
- Generate user action patterns
- Start load testing with 100 concurrent users

---

## 🔍 How It Works

### **Step 1: Framework Detection**
The scanner checks HTTP headers to identify your framework:
```
X-Powered-By: Express      → Express.js detected
X-Powered-By: Fastify      → Fastify detected  
Server: NestJS             → NestJS detected
```

### **Step 2: Route Discovery**

#### **Method A: OpenAPI/Swagger** (Preferred)
Checks common locations:
- `/openapi.json`
- `/swagger.json`
- `/api/openapi.json`
- `/docs/swagger.json`

If found, parses all routes automatically!

#### **Method B: Common Endpoints**
Tests standard endpoints:
```
GET  /
GET  /health
GET  /api/health
GET  /api/users
POST /api/login
GET  /api/products
...and 20+ more
```

#### **Method C: Intelligent Fuzzing**
Tries common resource names with patterns:
```
/api/users
/api/posts
/api/items
/api/v1/users
/v1/products
...etc
```

### **Step 3: Action Generation**
Groups discovered routes into realistic user journeys:
```
Discovered Routes:
  GET  /api/users
  GET  /api/users/:id
  POST /api/users
  
Generated Actions:
  "Browse users" (GET /api/users, GET /api/users/:id)
  "Create user" (POST /api/users)
```

---

## 📊 Example Output

```bash
$ ./traffic-sim -url http://localhost:3000 -scan -users 50

🔍 Starting backend discovery...

🔍 Scanning backend for routes: http://localhost:3000
   Detected framework: express
   Found OpenAPI spec at: /openapi.json
✅ Discovered 23 routes

✅ Generated 8 user action patterns from discovered routes

🎯 Traffic Simulator v1.1.0
============================

🚀 Starting traffic simulation...
   Target: http://localhost:3000
   Concurrent Users: 50
   Duration: 2m0s
   
📊 [12:00:05] Users: 50 | Requests: 312 | Success: 98.7% | Avg RT: 145ms | RPS: 62.4
📊 [12:00:10] Users: 50 | Requests: 625 | Success: 98.4% | Avg RT: 152ms | RPS: 62.5
```

---

## ⚙️ Advanced Usage

### **With Authentication**
```bash
./traffic-sim -url http://localhost:3000 \
  -scan \
  -auth "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -users 100
```

### **Custom Timeout**
```bash
./traffic-sim -url http://localhost:3000 \
  -scan \
  -timeout 30s \
  -users 200
```

### **Skip Certain Paths**
(Coming soon - config file support)

---

## 🎯 Supported Frameworks

### **Node.js** ✅
- Express.js
- Fastify
- NestJS
- Koa
- Hapi

### **Python** 🟡 (Partial)
- Django (via OpenAPI)
- Flask (via OpenAPI)
- FastAPI (via OpenAPI) ✅

### **Go** 🟡 (Partial)
- Gin (via OpenAPI)
- Echo (via OpenAPI)
- Fiber (via OpenAPI)

### **Other** 
- Any framework with OpenAPI/Swagger docs ✅

**Legend**: ✅ Full support | 🟡 Via OpenAPI only

---

## 💡 Tips for Best Results

### **1. Enable OpenAPI Documentation**
Most frameworks support this:

**Express + Swagger:**
```javascript
const swaggerUi = require('swagger-ui-express');
app.use('/api-docs', swaggerUi.serve, swaggerUi.setup(swaggerSpec));
```

**FastAPI:**
```python
from fastapi import FastAPI
app = FastAPI(openapi_url="/openapi.json")
```

**NestJS:**
```typescript
import { SwaggerModule } from '@nestjs/swagger';
SwaggerModule.setup('api', app, document);
```

### **2. Use Consistent Naming**
```
✅ Good: /api/users, /api/posts, /api/comments
❌ Bad: /getUsers, /list_posts, /commentsV2
```

### **3. Include Health Endpoints**
```javascript
app.get('/health', (req, res) => {
  res.json({ status: 'ok' });
});
```

### **4. Set Proper HTTP Methods**
```javascript
// ✅ Clear intent
app.get('/users', getUsers);
app.post('/users', createUser);
app.put('/users/:id', updateUser);

// ❌ Ambiguous
app.all('/users', handleUsers);
```

---

## 🔧 Troubleshooting

### **"No routes discovered"**

**Cause**: Backend doesn't have OpenAPI docs and uses non-standard paths

**Solutions**:
1. Add OpenAPI documentation (recommended)
2. Use default actions: Remove `-scan` flag
3. Create custom config file (coming soon)

### **"Detected framework: unknown"**

**Normal** - The scanner couldn't identify your framework but will still work via OpenAPI or fuzzing.

### **"All requests failing"**

**Check**:
1. Is your backend running?
2. Is the URL correct?
3. Do you need authentication? Use `-auth` flag

---

## 📈 Performance Impact

**Scanning Phase**:
- Duration: 2-10 seconds (depends on backend size)
- Requests: 50-200 (one-time)
- Impact: Minimal

**Testing Phase**:
- Same as regular traffic simulator
- Configurable via `-users` and `-duration`

---

## 🆚 vs Manual Configuration

| Aspect | Auto-Scan | Manual Config |
|--------|-----------|---------------|
| Setup Time | 0 seconds | 5-30 minutes |
| Accuracy | 80-95% | 100% |
| Maintenance | Automatic | Manual updates needed |
| Best For | Quick tests, CI/CD | Production load tests |

**Recommendation**: Use auto-scan for development, manual config for production testing.

---

## 🚧 Coming Soon

- [ ] Configuration file support (JSON/YAML)
- [ ] Skip/exclude specific paths
- [ ] Custom header injection
- [ ] Route weighting adjustment
- [ ] Save/load discovered routes
- [ ] Export to Postman collection
- [ ] GraphQL endpoint support
- [ ] WebSocket support

---

## 💻 Code Example: Adding OpenAPI to Express

```javascript
const express = require('express');
const swaggerJsdoc = require('swagger-jsdoc');
const swaggerUi = require('swagger-ui-express');

const app = express();

// Swagger configuration
const options = {
  definition: {
    openapi: '3.0.0',
    info: {
      title: 'My API',
      version: '1.0.0',
    },
  },
  apis: ['./routes/*.js'], // Path to files with JSDoc comments
};

const specs = swaggerJsdoc(options);

// Serve OpenAPI JSON
app.get('/openapi.json', (req, res) => {
  res.json(specs);
});

// Serve Swagger UI (optional)
app.use('/api-docs', swaggerUi.serve, swaggerUi.setup(specs));

// Your routes with JSDoc comments
/**
 * @swagger
 * /users:
 *   get:
 *     summary: Get all users
 *     tags: [Users]
 */
app.get('/users', getUsers);

app.listen(3000);
```

Now run:
```bash
./traffic-sim -url http://localhost:3000 -scan
```

It will automatically discover your `/users` endpoint!

---

**Built with ❤️ for developers who want to test before breaking production**
