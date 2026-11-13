import { z } from 'zod';

export const teamSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
});

export const personSchema = z.object({
  id: z.string(),
  full_name: z.string(),
});

export const progressSchema = z.object({
  done: z.number(),
  total: z.number(),
  pending: z.number(),
  in_progress: z.number(),
  blocked: z.number(),
  skipped: z.number(),
  percent: z.number(),
});

export const paceSchema = z.object({
  expected: z.number(),
  completed: z.number(),
  percent: z.number(),
});

export const blockerSchema = z.object({
  item_id: z.string(),
  title: z.string(),
  reason: z.string(),
});

export const overdueSchema = z.object({
  item_id: z.string(),
  title: z.string(),
  due_on: z.string(),
  days_overdue: z.number(),
});

export const activitySchema = z.object({
  questions: z.number(),
  last_event_at: z.string().nullable().optional(),
});

export const memberSchema = z.object({
  id: z.string(),
  full_name: z.string(),
  email: z.string(),
  position: z.string(),
  team: teamSchema,
  mentor: personSchema.nullable().optional(),
  plan: z
    .object({
      id: z.string(),
      title: z.string(),
      starts_on: z.string(),
      current_stage: z.string(),
    })
    .nullable()
    .optional(),
  progress: progressSchema,
  pace: paceSchema,
  blockers: z.array(blockerSchema),
  overdue: z.array(overdueSchema),
  activity: activitySchema,
  at_risk: z.boolean(),
});

export const teamStatSchema = z.object({
  id: z.string(),
  name: z.string(),
  interns: z.number(),
  avg_progress: z.number(),
  avg_pace: z.number(),
  blocked: z.number(),
  overdue: z.number(),
});

export const overviewSchema = z.object({
  interns: z.number(),
  avg_progress: z.number(),
  avg_pace: z.number(),
  blocked: z.number(),
  overdue: z.number(),
  at_risk: z.number(),
  teams: z.array(teamStatSchema),
});

export const reportSchema = z.object({
  interns: z.array(memberSchema),
  overview: overviewSchema,
});

export type TeamMember = z.infer<typeof memberSchema>;
export type TeamOverview = z.infer<typeof overviewSchema>;
export type TeamReport = z.infer<typeof reportSchema>;
