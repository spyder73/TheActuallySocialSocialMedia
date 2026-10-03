ALTER TABLE "User"
    ADD COLUMN "role" TEXT NOT NULL DEFAULT 'user',
    ADD COLUMN "disabledAt" TIMESTAMP(3);

ALTER TABLE "User" ADD CONSTRAINT "User_role_check" CHECK ("role" IN ('user', 'admin'));

CREATE TABLE "AccountToken" (
    "id" TEXT PRIMARY KEY,
    "tokenHash" BYTEA NOT NULL UNIQUE,
    "purpose" TEXT NOT NULL CHECK ("purpose" IN ('invite', 'reset')),
    "inviteRole" TEXT NOT NULL DEFAULT 'user' CHECK ("inviteRole" IN ('user', 'admin')),
    "email" TEXT NOT NULL,
    "createdById" TEXT REFERENCES "User"("id") ON DELETE SET NULL,
    "userId" TEXT REFERENCES "User"("id") ON DELETE CASCADE,
    "expiresAt" TIMESTAMPTZ NOT NULL,
    "usedAt" TIMESTAMPTZ,
    "createdAt" TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX "AccountToken_email_purpose_idx" ON "AccountToken" (lower("email"), "purpose");
CREATE INDEX "AccountToken_expiresAt_idx" ON "AccountToken" ("expiresAt");

CREATE TABLE "AuthSession" (
    "id" TEXT PRIMARY KEY,
    "tokenHash" BYTEA NOT NULL UNIQUE,
    "userId" TEXT NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "createdAt" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "expiresAt" TIMESTAMPTZ NOT NULL,
    "revokedAt" TIMESTAMPTZ
);
CREATE INDEX "AuthSession_userId_idx" ON "AuthSession" ("userId");
CREATE INDEX "AuthSession_expiresAt_idx" ON "AuthSession" ("expiresAt");

-- Login and operator account lookup are case-insensitive. Fail closed on an
-- ambiguous legacy dataset rather than permitting a shadow account.
CREATE UNIQUE INDEX "User_email_lower_key" ON "User" (lower("email"));
CREATE UNIQUE INDEX "User_username_lower_key" ON "User" (lower("username"));
