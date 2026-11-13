import { test, expect } from "bun:test";
import { Database } from "bun:sqlite";
import { CREATE, acquireSQL, releaseSQL } from "./lease.ts";

test("only one publisher acquires a live lease; expiration and owner-specific release work", () => {
  const db = new Database(":memory:");
  try {
    db.exec(CREATE);
    expect(db.query(acquireSQL("first")).all()).toEqual([{ owner: "first" }]);
    expect(db.query(acquireSQL("second")).all()).toEqual([]);
    db.exec(releaseSQL("second"));
    expect(db.query(acquireSQL("second")).all()).toEqual([]);
    db.exec("UPDATE __pairfob_fork_deploy_lease SET expires_at=unixepoch()-1");
    expect(db.query(acquireSQL("second")).all()).toEqual([{ owner: "second" }]);
    db.exec(releaseSQL("first"));
    expect(db.query(acquireSQL("third")).all()).toEqual([]);
    db.exec(releaseSQL("second"));
    expect(db.query(acquireSQL("third")).all()).toEqual([{ owner: "third" }]);
  } finally { db.close(); }
});
