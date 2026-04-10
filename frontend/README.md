# Skat Frontend

React frontend for the Skat card game.

## Development

```bash
# Install dependencies
npm install

# Start development server (proxies API requests to localhost:8080)
npm start

# Build for production
npm run build
```

## Running the Full Stack

1. Start the Go backend:
```bash
cd ..
go run cmd/server/main.go
```

2. In another terminal, start the React dev server:
```bash
cd frontend
npm start
```

3. Open http://localhost:3000

## Production Build

1. Build the React app:
```bash
cd frontend
npm run build
```

2. Start the Go server (serves the React build):
```bash
cd ..
go run cmd/server/main.go
```

3. Open http://localhost:8080
