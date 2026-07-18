import { defineSchema, defineTable } from "convex/server";
import { v } from "convex/values";

export default defineSchema({
  rooms: defineTable({ name: v.string(), repo: v.string(), defaultBranch: v.string(), joinCode: v.string(), checkCommand: v.optional(v.string()), createdAt: v.number() }).index("by_join_code", ["joinCode"]),
  participants: defineTable({ roomId: v.id("rooms"), name: v.string(), agent: v.string(), tokenHash: v.string(), joinedAt: v.number(), branch: v.optional(v.string()), head: v.optional(v.string()), dirty: v.optional(v.boolean()), changedPaths: v.optional(v.array(v.string())) }).index("by_token_hash", ["tokenHash"]).index("by_room", ["roomId"]),
  intents: defineTable({ roomId: v.id("rooms"), participantId: v.id("participants"), task: v.string(), objective: v.optional(v.string()), expectedPaths: v.array(v.string()), status: v.string(), updatedAt: v.number() }).index("by_participant", ["participantId"]).index("by_room", ["roomId"]),
  decisions: defineTable({ roomId: v.id("rooms"), participantId: v.id("participants"), title: v.string(), body: v.string(), createdAt: v.number() }).index("by_room", ["roomId"]),
  events: defineTable({ roomId: v.id("rooms"), name: v.string(), payload: v.any(), createdAt: v.number() }).index("by_room", ["roomId"]),
});
