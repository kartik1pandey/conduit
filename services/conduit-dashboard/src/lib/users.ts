import { pool } from "@/lib/db";

export type Role = "owner" | "developer" | "read-only";

export type User = {
  id: string;
  merchantId: string;
  email: string;
  passwordHash: string | null;
  role: Role;
  createdAt: Date;
};

function fromRow(row: {
  id: string;
  merchant_id: string;
  email: string;
  password_hash: string | null;
  role: string;
  created_at: Date;
}): User {
  return {
    id: row.id,
    merchantId: row.merchant_id,
    email: row.email,
    passwordHash: row.password_hash,
    role: row.role as Role,
    createdAt: row.created_at,
  };
}

export async function getUserByEmail(email: string): Promise<User | null> {
  const { rows } = await pool.query(
    "SELECT id, merchant_id, email, password_hash, role, created_at FROM users WHERE email = $1",
    [email],
  );
  return rows[0] ? fromRow(rows[0]) : null;
}

export async function getUserById(id: string): Promise<User | null> {
  const { rows } = await pool.query(
    "SELECT id, merchant_id, email, password_hash, role, created_at FROM users WHERE id = $1",
    [id],
  );
  return rows[0] ? fromRow(rows[0]) : null;
}

// createOwner is the one path that creates a merchant's first dashboard
// user — it's always role "owner" because holding the merchant's own
// sk_test_... key (verified against conduit-core before this is ever
// called) is proof of full account control, not just team membership.
export async function createOwner(
  merchantId: string,
  email: string,
  passwordHash: string,
): Promise<User> {
  const { rows } = await pool.query(
    `INSERT INTO users (merchant_id, email, password_hash, role)
     VALUES ($1, $2, $3, 'owner')
     RETURNING id, merchant_id, email, password_hash, role, created_at`,
    [merchantId, email, passwordHash],
  );
  return fromRow(rows[0]);
}

export class EmailAlreadyExistsError extends Error {
  constructor() {
    super("a dashboard account with this email already exists");
  }
}

// inviteUser is the owner-only path that adds a teammate — the caller (a
// server action) must have already checked the OPA policy for
// action="invite_user" before this is ever reached; this function itself
// has no notion of roles-that-can-invite, only "insert this row."
//
// passwordHash is optional: with no email-sending infrastructure in this
// test-mode project, an owner either sets an initial password directly
// (told to the teammate out-of-band) or leaves it unset, in which case the
// account is GitHub-OAuth-only until they log in that way once.
export async function inviteUser(
  merchantId: string,
  email: string,
  role: "developer" | "read-only",
  passwordHash?: string,
): Promise<User> {
  try {
    const { rows } = await pool.query(
      `INSERT INTO users (merchant_id, email, password_hash, role)
       VALUES ($1, $2, $3, $4)
       RETURNING id, merchant_id, email, password_hash, role, created_at`,
      [merchantId, email, passwordHash ?? null, role],
    );
    return fromRow(rows[0]);
  } catch (err: unknown) {
    if (
      err instanceof Error &&
      "code" in err &&
      (err as { code: string }).code === "23505"
    ) {
      throw new EmailAlreadyExistsError();
    }
    throw err;
  }
}

export async function setPassword(
  userId: string,
  passwordHash: string,
): Promise<void> {
  await pool.query("UPDATE users SET password_hash = $1 WHERE id = $2", [
    passwordHash,
    userId,
  ]);
}

export async function listUsersForMerchant(
  merchantId: string,
): Promise<User[]> {
  const { rows } = await pool.query(
    "SELECT id, merchant_id, email, password_hash, role, created_at FROM users WHERE merchant_id = $1 ORDER BY created_at",
    [merchantId],
  );
  return rows.map(fromRow);
}
