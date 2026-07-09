# QA Testing Guide

This guide is for testers who are new to API testing.

## What is a token and why do I need it?

The API is protected. Before you can make any request (except `/health`), you need to prove who you are. You do this by getting a **token** from Keycloak (our authentication service) and attaching it to every request.

Think of it like a badge — you get it at the entrance, then show it every time you go through a door.

A token looks like a long random string:
```
eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0...
```

Tokens expire after **5 minutes**. If you get a `401 Unauthorized` error, just get a new token.

---

## Step 1 — Create a test user in Keycloak

1. Open [http://localhost:8180](http://localhost:8180) in your browser
2. Log in with `admin` / `admin`
3. Make sure the `appraisal` realm is selected (top left dropdown)
4. Go to **Users** → **Create user**
5. Fill in username, email, first name, last name → **Create**
6. Go to the **Credentials** tab → **Set password** → enter a password, turn off **Temporary** → **Save password**
7. Go to the **Role mapping** tab → **Assign role** → switch filter to **Filter by realm roles** → select a role (e.g. `client`) → **Assign**

---

## Step 2 — Get a token

### Using curl

Run this in the terminal (replace `<username>` and `<password>` with your test user's credentials):

```bash
curl -s -X POST http://localhost:8180/realms/appraisal/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=appraisal-frontend&username=<username>&password=<password>" \
  | jq -r .access_token
```

Copy the output — that's your token.

### Using Postman

1. Create a new request → method `POST`
2. URL: `http://localhost:8180/realms/appraisal/protocol/openid-connect/token`
3. Go to the **Body** tab → select `x-www-form-urlencoded`
4. Add these fields:

| Key            | Value                  |
|----------------|------------------------|
| `grant_type`   | `password`             |
| `client_id`    | `appraisal-frontend`   |
| `username`     | your test username     |
| `password`     | your test password     |

5. Click **Send** — copy the `access_token` value from the response

---

## Step 3 — Make API requests

### Using Swagger UI

1. Open [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
2. Click **Authorize** (top right, lock icon)
3. In the `BearerAuth` field enter your token (without the word "Bearer")
4. Click **Authorize** → **Close**
5. Now you can expand any endpoint and click **Try it out** → **Execute**

### Using curl

Attach the token with `-H "Authorization: Bearer <token>"`:

```bash
# Get token and save it to a variable
TOKEN=$(curl -s -X POST http://localhost:8180/realms/appraisal/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=appraisal-frontend&username=<username>&password=<password>" \
  | jq -r .access_token)

# Create a request (email and phone_number are required)
curl -s -X POST http://localhost:8080/requests \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"email": "test@example.com", "phone_number": "+79161234567", "object_type": "apartment", "address": "Moscow, Lenina 1"}'
```

### Using Postman

1. Create a new request
2. Go to the **Authorization** tab → Type: `Bearer Token`
3. Paste your token in the **Token** field
4. Set the method, URL, and body as needed → **Send**

---

## What to test

### Roles and access control

| Scenario | Expected result |
|---|---|
| Request without a token | `401 Unauthorized` |
| `client` creates a request | `201 Created` |
| `appraiser` tries to create a request | `403 Forbidden` |
| `client` views their own request | `200 OK` |
| `client` tries to view someone else's request | `403 Forbidden` |
| `appraiser` views any request | `200 OK` |
| `appraiser` changes request status | `200 OK` |
| `client` tries to change status | `403 Forbidden` |

### Status flow

Statuses must change strictly in order — skipping is not allowed:

```
new → in_progress → inspection_scheduled → inspection_completed → appraisal → report_sent → closed
```

| Scenario | Expected result |
|---|---|
| Move from `new` to `in_progress` | `200 OK` |
| Move from `new` to `closed` (skip) | `422 Unprocessable Entity` |
| Move backwards (e.g. `in_progress` to `new`) | `422 Unprocessable Entity` |

### Required and optional fields

When creating a request, `email` and `phone_number` are **required**; `object_type` and `address` are optional — the appraiser can fill them in later.

| Scenario | Expected result |
|---|---|
| Create with empty body `{}` | `400 Bad Request` |
| Create with only `email` + `phone_number` | `201 Created` |
| Create with `object_type` and `address` as well | `201 Created` |

### Concurrent changes (optimistic locking)

If two people modify the same request at the same time, the loser gets `409 Conflict` — re-read the request and retry.

| Scenario | Expected result |
|---|---|
| Two simultaneous status changes | one `200 OK`, one `409 Conflict` |
| Simultaneous field PATCHes | one `200 OK`, the rest `409 Conflict` |
