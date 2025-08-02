# OTC API Server

Standalone API server for the OTC Markets Scoring system.

## Endpoints

- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login 
- `POST /api/v1/upload/csv` - Upload company CSV
- `GET /api/v1/companies` - List companies
- `POST /api/v1/scoring/companies/:id/score` - Score company
- `GET /api/v1/health` - Health check

## Deployment

This repository is configured for easy Railway deployment:

1. Push to GitHub
2. Connect to Railway
3. Set environment variables
4. Deploy!

## Environment Variables

```env
DATABASE_URL=postgresql://...
JWT_SECRET=your-secret
OXYLABS_USERNAME=username
OXYLABS_PASSWORD=password
```