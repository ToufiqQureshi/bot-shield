# HakaiShield Backend Wiring Documentation

## Overview

This document provides complete backend wiring instructions for connecting the frontend to backend services. It covers every page, feature, button, and data flow.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Authentication System](#authentication-system)
3. [API Endpoints](#api-endpoints)
4. [Data Models](#data-models)
5. [Page-by-Page Wiring Guide](#page-by-page-wiring-guide)
6. [Real-time Features](#real-time-features)
7. [Payment Integration](#payment-integration)
8. [Email Services](#email-services)
9. [File Upload & Storage](#file-upload--storage)
10. [Error Handling](#error-handling)

---

## Architecture Overview

### Tech Stack Recommendation

**Backend:**
- Node.js + Express OR Go (since proxy is in Go)
- PostgreSQL (primary database)
- Redis (caching, sessions, rate limiting)
- JWT (authentication)
- Stripe (payments)
- SendGrid/AWS SES (emails)
- AWS S3 (file storage)

**Infrastructure:**
- Docker containers
- Nginx reverse proxy
- SSL/TLS certificates
- CDN for static assets

### Environment Variables

```bash
# Database
DATABASE_URL=postgresql://user:pass@localhost:5432/hakaishield
REDIS_URL=redis://localhost:6379

# Authentication
JWT_SECRET=your-super-secret-key
JWT_EXPIRES_IN=7d

# Stripe
STRIPE_SECRET_KEY=sk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...

# Email
SENDGRID_API_KEY=SG...
FROM_EMAIL=noreply@hakaishield.io

# AWS
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
AWS_S3_BUCKET=hakaishield-uploads

# App
NODE_ENV=production
API_URL=https://api.hakaishield.io
FRONTEND_URL=https://app.hakaishield.io
```

---

## Authentication System

### Flow Diagram

```
User → Sign Up → Verify Email → Onboarding → Dashboard
  ↓
User → Sign In → Get JWT → Dashboard
  ↓
User → Forgot Password → Reset Email → New Password
```

### JWT Token Structure

```javascript
{
  "userId": "uuid",
  "email": "user@example.com",
  "role": "user" | "admin",
  "tenantId": "uuid",
  "iat": 1234567890,
  "exp": 1234567890
}
```

### Middleware

```javascript
// auth.middleware.js
const authenticateToken = (req, res, next) => {
  const authHeader = req.headers['authorization'];
  const token = authHeader && authHeader.split(' ')[1];
  
  if (!token) return res.sendStatus(401);
  
  jwt.verify(token, process.env.JWT_SECRET, (err, user) => {
    if (err) return res.sendStatus(403);
    req.user = user;
    next();
  });
};
```

---

## API Endpoints

### Base URL

```
Production: https://api.hakaishield.io/v1
Development: http://localhost:3000/v1
```

### Authentication Endpoints

#### POST /auth/signup
**Purpose:** Create new user account

**Request:**
```javascript
{
  "name": "John Doe",
  "email": "john@example.com",
  "password": "securePassword123",
  "company": "Acme Inc" // optional
}
```

**Response (201):**
```javascript
{
  "success": true,
  "data": {
    "userId": "uuid",
    "email": "john@example.com",
    "verificationToken": "token123"
  }
}
```

**Backend Actions:**
1. Validate email format
2. Check if email already exists
3. Hash password (bcrypt, 10 rounds)
4. Create user in database
5. Generate verification token
6. Send verification email
7. Return user data

**Database Query:**
```sql
INSERT INTO users (id, name, email, password_hash, company, verified, created_at)
VALUES (uuid_generate_v4(), $1, $2, $3, $4, false, NOW())
RETURNING id, email;
```

---

#### POST /auth/verify-email
**Purpose:** Verify email address

**Request:**
```javascript
{
  "token": "verification-token-123"
}
```

**Response (200):**
```javascript
{
  "success": true,
  "message": "Email verified successfully"
}
```

**Backend Actions:**
1. Find user by verification token
2. Mark user as verified
3. Clear verification token
4. Return success

---

#### POST /auth/signin
**Purpose:** Authenticate user

**Request:**
```javascript
{
  "email": "john@example.com",
  "password": "securePassword123"
}
```

**Response (200):**
```javascript
{
  "success": true,
  "data": {
    "token": "jwt-token-here",
    "user": {
      "id": "uuid",
      "email": "john@example.com",
      "name": "John Doe",
      "onboardingComplete": false
    }
  }
}
```

**Backend Actions:**
1. Find user by email
2. Verify password hash
3. Check if email is verified
4. Generate JWT token
5. Return token and user data

---

#### POST /auth/forgot-password
**Purpose:** Request password reset

**Request:**
```javascript
{
  "email": "john@example.com"
}
```

**Response (200):**
```javascript
{
  "success": true,
  "message": "Password reset email sent"
}
```

**Backend Actions:**
1. Find user by email
2. Generate reset token (expires in 1 hour)
3. Store reset token in database
4. Send reset email with link: `https://app.hakaishield.io/reset-password?token=xxx`

---

#### POST /auth/reset-password
**Purpose:** Reset password with token

**Request:**
```javascript
{
  "token": "reset-token-123",
  "newPassword": "newSecurePassword123"
}
```

**Response (200):**
```javascript
{
  "success": true,
  "message": "Password reset successfully"
}
```

**Backend Actions:**
1. Find user by reset token
2. Check if token is expired
3. Hash new password
4. Update password in database
5. Clear reset token
6. Invalidate all existing sessions

---

#### GET /auth/me
**Purpose:** Get current user data

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200):**
```javascript
{
  "success": true,
  "data": {
    "id": "uuid",
    "email": "john@example.com",
    "name": "John Doe",
    "company": "Acme Inc",
    "onboardingComplete": true,
    "subscription": {
      "plan": "growth",
      "status": "active",
      "currentPeriodEnd": "2026-10-01T00:00:00Z"
    }
  }
}
```

---

### Onboarding Endpoints

#### POST /onboarding/complete
**Purpose:** Save onboarding data

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request:**
```javascript
{
  "useCase": "ecommerce",
  "monthlyVisitors": "500k-2m",
  "website": "https://example.com",
  "teamSize": "11-50",
  "botProblem": "pricing-scraping"
}
```

**Response (200):**
```javascript
{
  "success": true,
  "message": "Onboarding completed"
}
```

**Backend Actions:**
1. Validate data
2. Update user record with onboarding data
3. Mark onboarding as complete
4. Create tenant/domain record
5. Return success

**Database Query:**
```sql
UPDATE users 
SET onboarding_complete = true,
    use_case = $1,
    monthly_visitors = $2,
    website = $3,
    team_size = $4,
    bot_problem = $5,
    updated_at = NOW()
WHERE id = $6;
```

---

### Subscription Endpoints

#### GET /subscription/current
**Purpose:** Get current subscription details

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200):**
```javascript
{
  "success": true,
  "data": {
    "plan": "growth",
    "status": "active",
    "currentPeriodStart": "2026-09-01T00:00:00Z",
    "currentPeriodEnd": "2026-10-01T00:00:00Z",
    "usage": {
      "requests": 4200000,
      "requestsLimit": 10000000,
      "domains": 7,
      "domainsLimit": 10
    },
    "billingHistory": [
      {
        "id": "inv_123",
        "date": "2026-09-01T00:00:00Z",
        "amount": 50000, // in cents
        "status": "paid",
        "invoiceUrl": "https://..."
      }
    ]
  }
}
```

**Backend Actions:**
1. Get user's subscription from Stripe
2. Get usage metrics from database
3. Get billing history from Stripe
4. Return combined data

---

#### POST /subscription/upgrade
**Purpose:** Upgrade subscription plan

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request:**
```javascript
{
  "plan": "growth", // or "starter", "enterprise"
  "paymentMethodId": "pm_123" // from Stripe
}
```

**Response (200):**
```javascript
{
  "success": true,
  "data": {
    "subscriptionId": "sub_123",
    "plan": "growth",
    "nextBillingDate": "2026-10-01T00:00:00Z"
  }
}
```

**Backend Actions:**
1. Validate plan exists
2. Create or update Stripe subscription
3. Update user's subscription in database
4. Send confirmation email
5. Return new subscription data

---

### Payment Endpoints

#### POST /payment/create-intent
**Purpose:** Create Stripe payment intent

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request:**
```javascript
{
  "plan": "growth",
  "amount": 50000 // in cents
}
```

**Response (200):**
```javascript
{
  "success": true,
  "data": {
    "clientSecret": "pi_123_secret_456"
  }
}
```

**Backend Actions:**
1. Create Stripe PaymentIntent
2. Return client secret for frontend

**Code:**
```javascript
const paymentIntent = await stripe.paymentIntents.create({
  amount: 50000,
  currency: 'usd',
  metadata: {
    userId: req.user.id,
    plan: 'growth'
  }
});

return { clientSecret: paymentIntent.client_secret };
```

---

#### POST /payment/webhook
**Purpose:** Handle Stripe webhooks

**Request:** Stripe webhook payload

**Backend Actions:**
1. Verify webhook signature
2. Handle different event types:
   - `payment_intent.succeeded` → Activate subscription
   - `payment_intent.payment_failed` → Send failure email
   - `customer.subscription.updated` → Update subscription
   - `customer.subscription.deleted` → Cancel subscription
   - `invoice.payment_succeeded` → Update billing history
   - `invoice.payment_failed` → Send dunning email

**Code:**
```javascript
app.post('/webhook', express.raw({type: 'application/json'}), (req, res) => {
  const sig = req.headers['stripe-signature'];
  const event = stripe.webhooks.constructEvent(req.body, sig, webhookSecret);
  
  switch (event.type) {
    case 'payment_intent.succeeded':
      handlePaymentSuccess(event.data.object);
      break;
    case 'invoice.payment_failed':
      handlePaymentFailure(event.data.object);
      break;
    // ... other cases
  }
  
  res.json({received: true});
});
```

---

### Dashboard Endpoints

#### GET /dashboard/stats
**Purpose:** Get dashboard statistics

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200):**
```javascript
{
  "success": true,
  "data": {
    "totalRequests": 2410000,
    "cleanTraffic": 1970000,
    "blocked": 187000,
    "challenged": 142000,
    "deceived": 98000,
    "trends": {
      "totalRequests": "+12.3%",
      "cleanTraffic": "+8.1%",
      "blocked": "+23.4%",
      "challenged": "-5.2%",
      "deceived": "+31.7%"
    }
  }
}
```

**Backend Actions:**
1. Query traffic logs for last 24 hours
2. Aggregate by decision type
3. Calculate trends (compare to previous 24h)
4. Return stats

**Database Query:**
```sql
SELECT 
  COUNT(*) as total,
  SUM(CASE WHEN decision = 'PASS' THEN 1 ELSE 0 END) as passed,
  SUM(CASE WHEN decision = 'BLOCK' THEN 1 ELSE 0 END) as blocked,
  SUM(CASE WHEN decision = 'CHALLENGE' THEN 1 ELSE 0 END) as challenged,
  SUM(CASE WHEN decision = 'DECEIVE' THEN 1 ELSE 0 END) as deceived
FROM traffic_logs
WHERE tenant_id = $1
  AND created_at > NOW() - INTERVAL '24 hours';
```

---

#### GET /dashboard/traffic-chart
**Purpose:** Get traffic data for chart

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Query Parameters:**
```
?period=24h // or 7d, 30d
```

**Response (200):**
```javascript
{
  "success": true,
  "data": [
    {
      "time": "00:00",
      "passed": 8500,
      "blocked": 850,
      "challenged": 600,
      "deceived": 400
    },
    // ... 23 more hours
  ]
}
```

**Backend Actions:**
1. Group traffic logs by hour
2. Count by decision type per hour
3. Return time series data

---

#### GET /dashboard/top-offenders
**Purpose:** Get top blocked IPs

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200):**
```javascript
{
  "success": true,
  "data": [
    {
      "ip": "185.220.101.42",
      "ja4": "t13d1516h2_8daaf6152771_0271d189196b",
      "asn": "AS9009 M247 Ltd",
      "geo": "DE",
      "blocked": 12847,
      "lastSeen": "2 min ago"
    }
  ]
}
```

**Backend Actions:**
1. Group blocked requests by IP
2. Count blocks per IP
3. Get JA4, ASN, geo for each IP
4. Sort by block count
5. Return top 10

---

#### GET /evidence-logs
**Purpose:** Get evidence logs with filters

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Query Parameters:**
```
?search=185.220.101.42
&decision=BLOCK
&startDate=2026-09-01
&endDate=2026-09-17
&page=1
&limit=50
&sort=timestamp
&order=desc
```

**Response (200):**
```javascript
{
  "success": true,
  "data": {
    "logs": [
      {
        "id": "log-1",
        "timestamp": "2026-09-17T14:23:01Z",
        "method": "GET",
        "path": "/api/v1/users",
        "ip": "185.220.101.42",
        "geo": "DE",
        "ja4": "t13d1516h2_8daaf6152771_0271d189196b",
        "signals": ["UA Mismatch", "TLS Fragmentation"],
        "score": 87,
        "decision": "BLOCK",
        "headers": {...},
        "tls": {...}
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 50,
      "total": 1234,
      "totalPages": 25
    }
  }
}
```

**Backend Actions:**
1. Build dynamic query based on filters
2. Apply search, date range, decision filters
3. Paginate results
4. Return logs with metadata

---

### Mitigation Rules Endpoints

#### GET /rules
**Purpose:** Get all mitigation rules

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200):**
```javascript
{
  "success": true,
  "data": {
    "managedRules": [
      {
        "id": "mr-1",
        "name": "Block Known Scrapers",
        "description": "Blocks requests from known scraping frameworks",
        "enabled": true,
        "hits": 23456
      }
    ],
    "customRules": [...],
    "exceptions": [...]
  }
}
```

---

#### POST /rules/custom
**Purpose:** Create custom rule

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request:**
```javascript
{
  "name": "Block suspicious JA4",
  "conditions": [
    {
      "field": "JA4 Fingerprint",
      "operator": "EQUALS",
      "value": "t13d1516h2_8daaf6152771_0271d189196b"
    },
    {
      "field": "Threat Score",
      "operator": ">",
      "value": "80"
    }
  ],
  "action": "BLOCK"
}
```

**Response (201):**
```javascript
{
  "success": true,
  "data": {
    "id": "rule-123",
    "name": "Block suspicious JA4",
    "conditions": [...],
    "action": "BLOCK",
    "createdAt": "2026-09-17T14:23:01Z"
  }
}
```

**Backend Actions:**
1. Validate rule structure
2. Check for conflicts with existing rules
3. Save rule to database
4. Update rule engine configuration
5. Return created rule

---

#### PUT /rules/:id/toggle
**Purpose:** Enable/disable rule

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request:**
```javascript
{
  "enabled": false
}
```

**Response (200):**
```javascript
{
  "success": true,
  "message": "Rule updated"
}
```

---

### Protection Settings Endpoints

#### GET /settings/protection
**Purpose:** Get protection settings

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200):**
```javascript
{
  "success": true,
  "data": {
    "blockThreshold": 90,
    "challengeThreshold": 50,
    "challengeType": "pow",
    "honeypotEnabled": true,
    "tarpitDelay": 30,
    "wafSettings": {
      "sqliProtection": true,
      "xssProtection": true,
      "rateLimitEnabled": true,
      "rateLimitRpm": 100
    }
  }
}
```

---

#### PUT /settings/protection
**Purpose:** Update protection settings

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request:**
```javascript
{
  "blockThreshold": 85,
  "challengeThreshold": 45,
  "challengeType": "captcha"
}
```

**Response (200):**
```javascript
{
  "success": true,
  "message": "Settings updated"
}
```

**Backend Actions:**
1. Validate thresholds (block > challenge)
2. Update settings in database
3. Update proxy configuration
4. Return success

---

### Domains & SIEM Endpoints

#### GET /domains
**Purpose:** Get all protected domains

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200):**
```javascript
{
  "success": true,
  "data": [
    {
      "id": "d-1",
      "domain": "api.example.com",
      "origin": "10.0.1.50:8080",
      "certStatus": "valid",
      "certExpiry": "2025-08-15",
      "status": "active",
      "requests": "2.4M/day"
    }
  ]
}
```

---

#### POST /domains
**Purpose:** Add new domain

**Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request:**
```javascript
{
  "domain": "new.example.com",
  "origin": "10.0.1.100:8080"
}
```

**Response (201):**
```javascript
{
  "success": true,
  "data": {
    "id": "d-2",
    "domain": "new.example.com",
    "verificationToken": "verify-123",
    "status": "pending_verification"
  }
}
```

**Backend Actions:**
1. Check if domain already exists
2. Create domain record
3. Generate verification token
4. Send DNS verification instructions
5. Return domain with verification status

---

#### GET /siem/integrations
**Purpose:** Get SIEM integrations

**Response (200):**
```javascript
{
  "success": true,
  "data": [
    {
      "id": "siem-1",
      "name": "Datadog",
      "status": "connected",
      "lastSync": "30s ago",
      "events": "1.2M"
    }
  ]
}
```

---

#### POST /siem/integrations/:id/connect
**Purpose:** Connect SIEM integration

**Request:**
```javascript
{
  "apiKey": "dd-api-key-123",
  "region": "us1"
}
```

**Response (200):**
```javascript
{
  "success": true,
  "message": "Integration connected"
}
```

**Backend Actions:**
1. Validate API key
2. Test connection
3. Save integration config
4. Start log streaming
5. Return success

---

### Contact Form Endpoints

#### POST /contact/founding-application
**Purpose:** Submit founding customer application

**Request:**
```javascript
{
  "name": "John Doe",
  "email": "john@company.com",
  "company": "Acme Inc",
  "website": "https://acme.com",
  "monthlyVisitors": "500k-2m",
  "botProblem": "pricing-scraping",
  "message": "We're losing revenue to scrapers..."
}
```

**Response (201):**
```javascript
{
  "success": true,
  "message": "Application submitted"
}
```

**Backend Actions:**
1. Validate all fields
2. Save application to database
3. Send confirmation email to applicant
4. Send notification email to admin
5. Return success

---

## Data Models

### Users Table

```sql
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name VARCHAR(255) NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  company VARCHAR(255),
  verified BOOLEAN DEFAULT false,
  verification_token VARCHAR(255),
  reset_token VARCHAR(255),
  reset_token_expires TIMESTAMP,
  onboarding_complete BOOLEAN DEFAULT false,
  use_case VARCHAR(50),
  monthly_visitors VARCHAR(50),
  website VARCHAR(255),
  team_size VARCHAR(50),
  bot_problem VARCHAR(50),
  stripe_customer_id VARCHAR(255),
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

### Subscriptions Table

```sql
CREATE TABLE subscriptions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID REFERENCES users(id),
  stripe_subscription_id VARCHAR(255),
  plan VARCHAR(50) NOT NULL, -- starter, growth, enterprise
  status VARCHAR(50) NOT NULL, -- active, canceled, past_due
  current_period_start TIMESTAMP,
  current_period_end TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

### Traffic Logs Table

```sql
CREATE TABLE traffic_logs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID REFERENCES users(id),
  timestamp TIMESTAMP DEFAULT NOW(),
  method VARCHAR(10),
  path TEXT,
  ip VARCHAR(45),
  geo VARCHAR(10),
  ja4 VARCHAR(255),
  signals JSONB,
  score INTEGER,
  decision VARCHAR(50),
  headers JSONB,
  tls JSONB,
  enforced BOOLEAN DEFAULT true
);

-- Index for fast queries
CREATE INDEX idx_traffic_logs_tenant_timestamp ON traffic_logs(tenant_id, timestamp DESC);
CREATE INDEX idx_traffic_logs_decision ON traffic_logs(decision);
CREATE INDEX idx_traffic_logs_ip ON traffic_logs(ip);
```

### Rules Table

```sql
CREATE TABLE rules (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID REFERENCES users(id),
  name VARCHAR(255) NOT NULL,
  type VARCHAR(50), -- managed, custom
  conditions JSONB NOT NULL,
  action VARCHAR(50) NOT NULL,
  enabled BOOLEAN DEFAULT true,
  hits INTEGER DEFAULT 0,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

### Domains Table

```sql
CREATE TABLE domains (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID REFERENCES users(id),
  domain VARCHAR(255) UNIQUE NOT NULL,
  origin VARCHAR(255) NOT NULL,
  cert_status VARCHAR(50),
  cert_expiry DATE,
  status VARCHAR(50),
  verification_token VARCHAR(255),
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

### SIEM Integrations Table

```sql
CREATE TABLE siem_integrations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID REFERENCES users(id),
  provider VARCHAR(50) NOT NULL, -- datadog, splunk, aws_s3, elastic
  config JSONB NOT NULL,
  status VARCHAR(50), -- connected, disconnected
  last_sync TIMESTAMP,
  events_count BIGINT DEFAULT 0,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

### Applications Table

```sql
CREATE TABLE applications (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name VARCHAR(255) NOT NULL,
  email VARCHAR(255) NOT NULL,
  company VARCHAR(255),
  website VARCHAR(255),
  monthly_visitors VARCHAR(50),
  bot_problem VARCHAR(50),
  message TEXT,
  status VARCHAR(50) DEFAULT 'pending', -- pending, approved, rejected
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

## Page-by-Page Wiring Guide

### 1. Landing Page (`/landing`)

**Buttons to Wire:**

#### "Get started" button (nav)
```javascript
onClick={() => navigate('/sign-up')}
```

#### "Start free trial" button (hero)
```javascript
onClick={() => navigate('/pricing')}
```

#### "Apply for founding access" button
```javascript
onClick={() => navigate('/contact')}
```

#### Interactive demo "Run through hakaishield" button
```javascript
onClick={async () => {
  setLoading(true);
  const response = await fetch('/api/v1/demo/analyze', {
    method: 'POST',
    body: JSON.stringify({ sampleType: selectedSample })
  });
  const data = await response.json();
  setDemoResult(data);
  setLoading(false);
}}
```

**Backend Endpoint Needed:**
```
POST /demo/analyze
```

---

### 2. Pricing Page (`/pricing`)

**Buttons to Wire:**

#### "Start free trial" buttons (all plans)
```javascript
onClick={() => {
  if (user) {
    navigate('/payment?plan=' + plan.name.toLowerCase());
  } else {
    navigate('/sign-up?plan=' + plan.name.toLowerCase());
  }
}}
```

#### "Talk to sales" button (Enterprise)
```javascript
onClick={() => navigate('/contact?type=enterprise')}
```

---

### 3. Sign Up Page (`/sign-up`)

**Form Submission:**
```javascript
const handleSubmit = async (e) => {
  e.preventDefault();
  setLoading(true);
  
  try {
    const response = await fetch('/api/v1/auth/signup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(formData)
    });
    
    const data = await response.json();
    
    if (data.success) {
      // Store token
      localStorage.setItem('token', data.data.token);
      
      // Redirect to onboarding
      navigate('/onboarding');
    } else {
      setError(data.message);
    }
  } catch (error) {
    setError('An error occurred');
  } finally {
    setLoading(false);
  }
};
```

---

### 4. Sign In Page (`/sign-in`)

**Form Submission:**
```javascript
const handleSubmit = async (e) => {
  e.preventDefault();
  setLoading(true);
  
  try {
    const response = await fetch('/api/v1/auth/signin', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(formData)
    });
    
    const data = await response.json();
    
    if (data.success) {
      localStorage.setItem('token', data.data.token);
      
      // Check if onboarding complete
      if (!data.data.user.onboardingComplete) {
        navigate('/onboarding');
      } else {
        navigate('/');
      }
    } else {
      setError(data.message);
    }
  } catch (error) {
    setError('Invalid credentials');
  } finally {
    setLoading(false);
  }
};
```

---

### 5. Onboarding Page (`/onboarding`)

**Form Submission (final step):**
```javascript
const handleSubmit = async (e) => {
  e.preventDefault();
  setLoading(true);
  
  try {
    const response = await fetch('/api/v1/onboarding/complete', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify(formData)
    });
    
    const data = await response.json();
    
    if (data.success) {
      navigate('/');
    }
  } catch (error) {
    setError('An error occurred');
  } finally {
    setLoading(false);
  }
};
```

---

### 6. Dashboard Overview (`/`)

**Data Fetching:**
```javascript
useEffect(() => {
  const fetchStats = async () => {
    const response = await fetch('/api/v1/dashboard/stats', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    });
    const data = await response.json();
    setStats(data.data);
  };
  
  const fetchTrafficChart = async () => {
    const response = await fetch('/api/v1/dashboard/traffic-chart?period=24h', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    });
    const data = await response.json();
    setChartData(data.data);
  };
  
  const fetchTopOffenders = async () => {
    const response = await fetch('/api/v1/dashboard/top-offenders', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    });
    const data = await response.json();
    setTopOffenders(data.data);
  };
  
  fetchStats();
  fetchTrafficChart();
  fetchTopOffenders();
}, []);
```

**Shadow Mode Toggle:**
```javascript
const handleToggleShadowMode = async () => {
  const response = await fetch('/api/v1/settings/mode', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('token')}`
    },
    body: JSON.stringify({ mode: shadowMode ? 'enforce' : 'shadow' })
  });
  
  if (response.ok) {
    setShadowMode(!shadowMode);
  }
};
```

---

### 7. Evidence Logs Page (`/evidence-logs`)

**Data Fetching with Filters:**
```javascript
const fetchLogs = async () => {
  const params = new URLSearchParams({
    search: searchQuery,
    decision: decisionFilter,
    page: currentPage,
    limit: 50,
    sort: sortField,
    order: sortDir
  });
  
  const response = await fetch(`/api/v1/evidence-logs?${params}`, {
    headers: {
      'Authorization': `Bearer ${localStorage.getItem('token')}`
    }
  });
  
  const data = await response.json();
  setLogs(data.data.logs);
  setTotalPages(data.data.pagination.totalPages);
};

useEffect(() => {
  fetchLogs();
}, [searchQuery, decisionFilter, currentPage, sortField, sortDir]);
```

**Row Expansion:**
```javascript
const handleRowClick = async (logId) => {
  if (expandedRow === logId) {
    setExpandedRow(null);
    return;
  }
  
  // Fetch full details if not already loaded
  if (!logDetails[logId]) {
    const response = await fetch(`/api/v1/evidence-logs/${logId}`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    });
    const data = await response.json();
    setLogDetails({ ...logDetails, [logId]: data.data });
  }
  
  setExpandedRow(logId);
};
```

---

### 8. Mitigation Rules Page (`/mitigation-rules`)

**Toggle Rule:**
```javascript
const handleToggleRule = async (ruleId) => {
  const response = await fetch(`/api/v1/rules/${ruleId}/toggle`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('token')}`
    },
    body: JSON.stringify({ enabled: !rule.enabled })
  });
  
  if (response.ok) {
    setRules(rules.map(r => 
      r.id === ruleId ? { ...r, enabled: !r.enabled } : r
    ));
  }
};
```

**Create Custom Rule:**
```javascript
const handleCreateRule = async () => {
  const response = await fetch('/api/v1/rules/custom', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('token')}`
    },
    body: JSON.stringify({
      name: ruleName,
      conditions: conditions,
      action: action
    })
  });
  
  const data = await response.json();
  
  if (data.success) {
    setRules([...rules, data.data]);
    setShowBuilder(false);
  }
};
```

---

### 9. Protection Settings Page (`/protection-settings`)

**Save Settings:**
```javascript
const handleSaveSettings = async () => {
  const response = await fetch('/api/v1/settings/protection', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('token')}`
    },
    body: JSON.stringify({
      blockThreshold,
      challengeThreshold,
      challengeType,
      honeypotEnabled,
      tarpitDelay,
      wafSettings: {
        sqliProtection,
        xssProtection,
        rateLimitEnabled,
        rateLimitRpm
      }
    })
  });
  
  if (response.ok) {
    setSuccessMessage('Settings saved');
    setTimeout(() => setSuccessMessage(null), 3000);
  }
};
```

---

### 10. Subscription Page (`/subscription`)

**Fetch Subscription:**
```javascript
useEffect(() => {
  const fetchSubscription = async () => {
    const response = await fetch('/api/v1/subscription/current', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    });
    const data = await response.json();
    setSubscription(data.data);
  };
  
  fetchSubscription();
}, []);
```

**Change Plan:**
```javascript
const handleChangePlan = async (newPlan) => {
  const response = await fetch('/api/v1/subscription/upgrade', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('token')}`
    },
    body: JSON.stringify({ plan: newPlan })
  });
  
  if (response.ok) {
    navigate('/payment?plan=' + newPlan);
  }
};
```

---

### 11. Payment Page (`/payment`)

**Create Payment Intent:**
```javascript
useEffect(() => {
  const createPaymentIntent = async () => {
    const response = await fetch('/api/v1/payment/create-intent', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify({
        plan: planFromUrl,
        amount: planAmounts[planFromUrl]
      })
    });
    
    const data = await response.json();
    setClientSecret(data.data.clientSecret);
  };
  
  createPaymentIntent();
}, []);
```

**Process Payment:**
```javascript
const handleSubmit = async (e) => {
  e.preventDefault();
  setProcessing(true);
  
  const { error, paymentIntent } = await stripe.confirmCardPayment(clientSecret, {
    payment_method: {
      card: cardElement,
      billing_details: {
        name: formData.name,
        address: {
          country: formData.country,
          postal_code: formData.zip
        }
      }
    }
  });
  
  if (error) {
    setError(error.message);
  } else if (paymentIntent.status === 'succeeded') {
    // Payment successful
    navigate('/?payment=success');
  }
  
  setProcessing(false);
};
```

---

## Real-time Features

### WebSocket Connection

**For live dashboard updates:**

```javascript
// Frontend
const ws = new WebSocket('wss://api.hakaishield.io/ws');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  
  switch (data.type) {
    case 'traffic_update':
      updateTrafficChart(data.payload);
      break;
    case 'new_log':
      addNewLog(data.payload);
      break;
    case 'stats_update':
      updateStats(data.payload);
      break;
  }
};
```

**Backend WebSocket Handler:**

```javascript
const WebSocket = require('ws');
const wss = new WebSocket.Server({ port: 8080 });

wss.on('connection', (ws, req) => {
  const token = extractTokenFromUrl(req.url);
  const user = verifyToken(token);
  
  // Subscribe user to their tenant's updates
  ws.tenantId = user.tenantId;
  
  ws.on('close', () => {
    // Clean up
  });
});

// Broadcast updates
function broadcastToTenant(tenantId, type, payload) {
  wss.clients.forEach(client => {
    if (client.tenantId === tenantId && client.readyState === WebSocket.OPEN) {
      client.send(JSON.stringify({ type, payload }));
    }
  });
}
```

---

## Payment Integration

### Stripe Setup

**Create Customer:**
```javascript
const createStripeCustomer = async (user) => {
  const customer = await stripe.customers.create({
    email: user.email,
    name: user.name,
    metadata: {
      userId: user.id
    }
  });
  
  await db.query(
    'UPDATE users SET stripe_customer_id = $1 WHERE id = $2',
    [customer.id, user.id]
  );
  
  return customer.id;
};
```

**Create Subscription:**
```javascript
const createSubscription = async (customerId, priceId) => {
  const subscription = await stripe.subscriptions.create({
    customer: customerId,
    items: [{ price: priceId }],
    payment_behavior: 'default_incomplete',
    expand: ['latest_invoice.payment_intent']
  });
  
  return subscription;
};
```

**Price IDs:**
```javascript
const PRICE_IDS = {
  starter: 'price_starter_monthly',
  growth: 'price_growth_monthly',
  enterprise: 'price_enterprise_monthly'
};
```

---

## Email Services

### Email Templates

**Verification Email:**
```javascript
const sendVerificationEmail = async (email, token) => {
  const verificationUrl = `${process.env.FRONTEND_URL}/verify-email?token=${token}`;
  
  await sendgrid.send({
    to: email,
    from: process.env.FROM_EMAIL,
    subject: 'Verify your email - hakaishield',
    html: `
      <h1>Welcome to hakaishield!</h1>
      <p>Please verify your email by clicking the link below:</p>
      <a href="${verificationUrl}">Verify Email</a>
    `
  });
};
```

**Password Reset Email:**
```javascript
const sendPasswordResetEmail = async (email, token) => {
  const resetUrl = `${process.env.FRONTEND_URL}/reset-password?token=${token}`;
  
  await sendgrid.send({
    to: email,
    from: process.env.FROM_EMAIL,
    subject: 'Reset your password - hakaishield',
    html: `
      <h1>Password Reset Request</h1>
      <p>Click the link below to reset your password:</p>
      <a href="${resetUrl}">Reset Password</a>
      <p>This link expires in 1 hour.</p>
    `
  });
};
```

**Welcome Email:**
```javascript
const sendWelcomeEmail = async (email, name) => {
  await sendgrid.send({
    to: email,
    from: process.env.FROM_EMAIL,
    subject: 'Welcome to hakaishield!',
    html: `
      <h1>Welcome, ${name}!</h1>
      <p>Your account is ready. Here's how to get started:</p>
      <ol>
        <li>Add your domain in the dashboard</li>
        <li>Point your DNS to our proxy</li>
        <li>Enable shadow mode to test</li>
        <li>Review the evidence logs</li>
      </ol>
      <a href="${process.env.FRONTEND_URL}">Go to Dashboard</a>
    `
  });
};
```

---

## Error Handling

### Standard Error Response Format

```javascript
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid email format",
    "details": {
      "field": "email",
      "value": "invalid-email"
    }
  }
}
```

### Error Codes

```javascript
const ERROR_CODES = {
  // Auth
  'INVALID_CREDENTIALS': 'Invalid email or password',
  'EMAIL_NOT_VERIFIED': 'Please verify your email first',
  'TOKEN_EXPIRED': 'Token has expired',
  'TOKEN_INVALID': 'Invalid token',
  
  // Validation
  'VALIDATION_ERROR': 'Validation failed',
  'EMAIL_EXISTS': 'Email already registered',
  
  // Subscription
  'PAYMENT_FAILED': 'Payment failed',
  'SUBSCRIPTION_CANCELED': 'Subscription is canceled',
  
  // Rate Limiting
  'RATE_LIMIT_EXCEEDED': 'Too many requests',
  
  // Server
  'INTERNAL_ERROR': 'Internal server error'
};
```

### Error Handling Middleware

```javascript
const errorHandler = (err, req, res, next) => {
  console.error(err);
  
  if (err.name === 'ValidationError') {
    return res.status(400).json({
      success: false,
      error: {
        code: 'VALIDATION_ERROR',
        message: err.message
      }
    });
  }
  
  if (err.name === 'UnauthorizedError') {
    return res.status(401).json({
      success: false,
      error: {
        code: 'UNAUTHORIZED',
        message: 'Authentication required'
      }
    });
  }
  
  res.status(500).json({
    success: false,
    error: {
      code: 'INTERNAL_ERROR',
      message: 'An unexpected error occurred'
    }
  });
};
```

---

## Deployment Checklist

### Pre-deployment

- [ ] Set up production database
- [ ] Configure environment variables
- [ ] Set up Stripe webhook endpoints
- [ ] Configure email service
- [ ] Set up SSL certificates
- [ ] Configure CORS for frontend domain
- [ ] Set up rate limiting
- [ ] Configure logging (Winston, etc.)
- [ ] Set up monitoring (Sentry, DataDog)
- [ ] Set up backups

### Security

- [ ] Enable HTTPS
- [ ] Set secure cookie flags
- [ ] Implement rate limiting
- [ ] Add CSRF protection
- [ ] Sanitize all inputs
- [ ] Use parameterized queries
- [ ] Implement proper error handling
- [ ] Set up security headers (CSP, HSTS, etc.)

### Performance

- [ ] Add database indexes
- [ ] Set up Redis caching
- [ ] Implement pagination for large queries
- [ ] Optimize N+1 queries
- [ ] Set up CDN for static assets
- [ ] Enable compression (gzip)

---

## Support

For questions or issues, contact:
- Email: toufiq@hakaishield.io
- GitHub: https://github.com/ToufiqQureshi/hakaishield

---

**Last Updated:** September 17, 2026
**Version:** 1.0.0
