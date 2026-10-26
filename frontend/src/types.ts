export type RoleKey = 'MEMBER' | 'RECEPTION' | 'TRAINER' | 'MANAGER' | 'ACCOUNTANT';

export type ScreenState = 'ready' | 'loading' | 'empty' | 'error';

export interface AuthUser {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  role: RoleKey;
  home_branch_id?: string;
}

export interface AuthTokens {
  access_token: string;
  refresh_token: string;
  access_expiry?: string;
}

export interface UserProfile {
  email: string;
  role: string;
  name: string;
  branch: string;
  badge: string;
}

export interface TurnstileLog {
  id: string;
  card: string;
  dir: 'IN' | 'OUT';
  status: 'GRANTED' | 'DENIED';
  reason: string;
  latency: string;
  time: string;
  branch: string;
}

export interface GymClass {
  id: string;
  name: string;
  trainer: string;
  time: string;
  capacity: number;
  booked: number;
  category: string;
  intensity: 'Düşük' | 'Orta' | 'Yüksek';
  myBooking: boolean;
}

export interface InvoiceItem {
  number: string;
  member: string;
  amount: string;
  status: 'PAID' | 'PENDING' | 'OVERDUE';
  date: string;
  branch: string;
}

export interface PTSession {
  id: string;
  client: string;
  package: string;
  slot: string;
  status: 'CONFIRMED' | 'COMPLETED' | 'CANCELLED';
  remaining: string;
}
