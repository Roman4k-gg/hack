import { z } from 'zod';

export const itemStatusSchema = z.enum([
  'pending',
  'in_progress',
  'done',
  'blocked',
  'skipped',
]);
export type ItemStatus = z.infer<typeof itemStatusSchema>;

const planItemSchema = z.object({
  id: z.string(),
  title: z.string(),
  description: z.string(),
  kind: z.enum(['task', 'doc', 'meeting', 'access']),
  status: itemStatusSchema,
  blocked_reason: z.string().nullable().optional(),
});

const planStageSchema = z.object({
  id: z.string(),
  title: z.string(),
  description: z.string(),
  position: z.number(),
  items: z.array(planItemSchema),
});

export const planSchema = z.object({
  intern: z.object({
    id: z.string(),
    full_name: z.string(),
    team_name: z.string(),
  }),
  stages: z.array(planStageSchema),
  progress: z.object({
    done: z.number(),
    total: z.number(),
    percent: z.number(),
  }),
  next_step: z
    .object({
      item_id: z.string(),
      title: z.string(),
      reason: z.string(),
    })
    .nullable(),
});

export type Plan = z.infer<typeof planSchema>;
