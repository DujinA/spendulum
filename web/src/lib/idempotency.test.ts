import { newIdempotencyKey } from "@/lib/idempotency";

describe("newIdempotencyKey", () => {
  it("makes a random UUIDv4 without crypto.randomUUID, which a plain-HTTP page lacks", () => {
    const randomUUID = vi.spyOn(crypto, "randomUUID").mockImplementation(() => {
      throw new TypeError("crypto.randomUUID is not a function");
    });

    const keys = Array.from({ length: 50 }, () => newIdempotencyKey());

    for (const key of keys) expect(key).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    expect(new Set(keys).size).toBe(keys.length);
    randomUUID.mockRestore();
  });
});
