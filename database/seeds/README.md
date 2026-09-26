# Seeds

Seeding is a Go command, not SQL, because the development password has to be bcrypt hashed:

```bash
make seed
```

It is idempotent and refuses to run when `APP_ENV=production`. See
`apps/backend/cmd/seed/main.go` for the data it loads.
