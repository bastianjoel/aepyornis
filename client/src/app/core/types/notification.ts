export type Notification = {
  id: number;
  type: string;
  meta: unknown;
  read_at: string;

  subject: string;
  msg: string;
};
