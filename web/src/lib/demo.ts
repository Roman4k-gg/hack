import { z } from 'zod';
import { ApiError } from '@/lib/api';


export async function withDemoFallback<T>(
  load: () => Promise<T>,
  demo: T,
): Promise<{ data: T; demo: boolean }> {
  try {
    return { data: await load(), demo: false };
  } catch (error) {
    if (
      error instanceof ApiError &&
      (error.status === 404 || error.status === 501)
    ) {
      return { data: demo, demo: true };
    }

    throw error;
  }
}

export function listOf<T extends z.ZodType>(item: T) {
  return z.union([
    z.array(item),
    z.object({ items: z.array(item) }).transform((value) => value.items),
  ]);
}