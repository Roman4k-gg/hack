import { z } from 'zod';

export const roleSchema = z.enum(['intern', 'mentor', 'hr', 'admin']);
export type Role = z.infer<typeof roleSchema>;

export const userSchema = z.object({
  id: z.string(),
  email: z.string(),
  full_name: z.string(),
  role: roleSchema,
  position: z.string().nullable().optional(),
  team_id: z.string().nullable().optional(),
  team_name: z.string().nullable().optional(),
  team_slug: z.string().nullable().optional(),
  mentor_id: z.string().nullable().optional(),
  mentor_name: z.string().nullable().optional(),
});
export type User = z.infer<typeof userSchema>;

export const loginResponseSchema = z.object({
  token: z.string(),
  token_type: z.string(),
  expires_at: z.string(),
  user: userSchema,
});
export type LoginResponse = z.infer<typeof loginResponseSchema>;
