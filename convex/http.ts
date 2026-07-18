import { httpRouter } from "convex/server";
import { httpAction } from "./_generated/server";
import { api } from "./_generated/api";

const http = httpRouter();
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { "content-type": "application/json" } });
const hash = async (value: string) => Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value))), b => b.toString(16).padStart(2, "0")).join("");
const auth = async (ctx: any, request: Request, roomId: string) => { const token = request.headers.get("authorization")?.replace(/^Bearer /, ""); if (!token) return null; const participant = await ctx.runQuery(api.rooms.participantByToken, { tokenHash: await hash(token) }); return participant?.roomId === roomId ? participant : null; };

http.route({ path: "/rooms", method: "POST", handler: httpAction(async (ctx, request) => {
  const body = await request.json();
  if (!body.name || !body.repo || !body.defaultBranch) return json({ error: "name, repo, and defaultBranch are required" }, 400);
  const joinCode = crypto.randomUUID().replaceAll("-", "").slice(0, 12);
  const id = await ctx.runMutation(api.rooms.create, { name: body.name, repo: body.repo, defaultBranch: body.defaultBranch, checkCommand: body.checkCommand, joinCode });
  return json({ id, name: body.name, repo: body.repo, defaultBranch: body.defaultBranch, joinCode }, 201);
}) });

http.route({ pathPrefix: "/rooms/", method: "POST", handler: httpAction(async (ctx, request) => {
  const [, , joinCode, action] = new URL(request.url).pathname.split("/");
  if (action !== "participants") return json({ error: "not found" }, 404);
  const room = await ctx.runQuery(api.rooms.byJoinCode, { joinCode });
  if (!room) return json({ error: "not found" }, 404);
  const body = await request.json(); const token = crypto.randomUUID().replaceAll("-", "") + crypto.randomUUID().replaceAll("-", "");
  const participantId = await ctx.runMutation(api.rooms.join, { roomId: room._id, name: body.name, agent: body.agent, tokenHash: await hash(token) });
  return json({ participant: { id: participantId, roomId: room._id, name: body.name, agent: body.agent }, token }, 201);
}) });

http.route({ pathPrefix: "/rooms/", method: "GET", handler: httpAction(async (ctx, request) => { const [, , roomId, action] = new URL(request.url).pathname.split("/"); if (action !== "snapshot" || !(await auth(ctx, request, roomId))) return json({ error: "unauthorized" }, 401); const snapshot = await ctx.runQuery(api.rooms.snapshot, { roomId: roomId as any }); const room = snapshot.room; if (!room) return json({ error: "not found" }, 404); return json({ room: { id: room._id, name: room.name, repo: room.repo, defaultBranch: room.defaultBranch, checkCommand: room.checkCommand, createdAt: new Date(room.createdAt).toISOString() }, participants: snapshot.participants.map(p => ({ id: p._id, roomId: p.roomId, name: p.name, agent: p.agent, joinedAt: new Date(p.joinedAt).toISOString() })), intents: snapshot.intents.map(i => ({ id: i._id, roomId: i.roomId, participantId: i.participantId, task: i.task, objective: i.objective ?? "", expectedPaths: i.expectedPaths, status: i.status, updatedAt: new Date(i.updatedAt).toISOString() })), decisions: snapshot.decisions.map(d => ({ id: d._id, roomId: d.roomId, participantId: d.participantId, title: d.title, body: d.body, createdAt: new Date(d.createdAt).toISOString() })), overlaps: [], checkpoints: [], integrationRuns: [], latestSequence: 0 }); }) });

http.route({ pathPrefix: "/rooms/", method: "PUT", handler: httpAction(async (ctx, request) => { const [, , roomId, action] = new URL(request.url).pathname.split("/"); const participant = await auth(ctx, request, roomId); if (!participant) return json({ error: "unauthorized" }, 401); const body = await request.json(); if (action === "intents") { await ctx.runMutation(api.rooms.setIntent, { roomId: roomId as any, participantId: participant._id, task: body.task, objective: body.objective, expectedPaths: body.expectedPaths ?? [], status: body.status ?? "planning" }); return json({}); } if (action === "decisions") { await ctx.runMutation(api.rooms.addDecision, { roomId: roomId as any, participantId: participant._id, title: body.title, body: body.body }); return json({}); } return json({ error: "not found" }, 404); }) });

export default http;
