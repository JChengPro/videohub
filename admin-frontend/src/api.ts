export type Video = {
  id: number;
  author_id: number;
  username: string;
  title: string;
  description: string;
  status: string;
  create_time: string;
  published_at?: string;
  review_reason?: string;
  processing_error?: string;
  play_url: string;
  cover_url: string;
  width: number;
  height: number;
  duration_millis: number;
};
export type Review = {
  id: number;
  reviewer_name: string;
  decision: string;
  reason: string;
  create_time: string;
};
export type Detail = { video: Video; reviews: Review[] };
const key = "videohub_review_session";
export function getToken() {
  return sessionStorage.getItem(key) ?? "";
}
export function setToken(token: string) {
  if (token) sessionStorage.setItem(key, token);
  else sessionStorage.removeItem(key);
}
export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}
export async function post<T>(path: string, body: unknown = {}): Promise<T> {
  const response = await fetch(`/api/admin/${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${getToken()}`,
    },
    body: JSON.stringify(body),
  });
  const text = await response.text();
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    throw new ApiError("服务暂时不可用，请稍后重试", response.status);
  }
  if (!response.ok) {
    if (response.status === 401 && !["login", "setup", "link/info", "link/accept"].includes(path)) {
      setToken("");
      window.dispatchEvent(new Event("review-session-ended"));
    }
    throw new ApiError(
      (data as { error?: string }).error ?? "请求失败",
      response.status,
    );
  }
  return data as T;
}
