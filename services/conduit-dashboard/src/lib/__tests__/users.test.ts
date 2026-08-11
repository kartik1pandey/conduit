import { beforeEach, describe, expect, it } from "vitest";
import { v4 as uuidv4 } from "uuid";
import { pool } from "@/lib/db";
import {
  EmailAlreadyExistsError,
  createOwner,
  getUserByEmail,
  inviteUser,
  listUsersForMerchant,
} from "@/lib/users";

const skip = !process.env.DASHBOARD_DATABASE_URL;

describe.skipIf(skip)("users", () => {
  beforeEach(async () => {
    await pool.query("TRUNCATE users");
  });

  it("createOwner always assigns the owner role", async () => {
    const merchantId = uuidv4();
    const user = await createOwner(merchantId, "owner@example.com", "hashed");
    expect(user.role).toBe("owner");
    expect(user.merchantId).toBe(merchantId);
  });

  it("inviteUser rejects a duplicate email", async () => {
    const merchantId = uuidv4();
    await inviteUser(merchantId, "dup@example.com", "developer");
    await expect(
      inviteUser(merchantId, "dup@example.com", "read-only"),
    ).rejects.toThrow(EmailAlreadyExistsError);
  });

  it("listUsersForMerchant is scoped per merchant", async () => {
    const merchantA = uuidv4();
    const merchantB = uuidv4();
    await createOwner(merchantA, "a-owner@example.com", "hashed");
    await inviteUser(merchantA, "a-dev@example.com", "developer");
    await createOwner(merchantB, "b-owner@example.com", "hashed");

    const usersA = await listUsersForMerchant(merchantA);
    expect(usersA).toHaveLength(2);
    expect(usersA.every((u) => u.merchantId === merchantA)).toBe(true);
  });

  it("getUserByEmail returns null for an unknown email", async () => {
    expect(await getUserByEmail("nobody@example.com")).toBeNull();
  });
});
