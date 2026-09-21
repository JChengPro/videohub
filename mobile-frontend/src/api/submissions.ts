import { request } from "./index";
export type Submission = {
  id: number;
  title: string;
  description: string;
  status: string;
  play_url: string;
  cover_url: string;
  create_time: string;
  published_at?: string;
  review_reason?: string;
  processing_error?: string;
};
export const statusLabel = (status: string) =>
  ({
    processing: "处理中",
    pending_review: "待审核",
    published: "已发布",
    rejected: "已驳回",
    failed: "处理失败",
    deleted: "已撤回",
  })[status] ?? status;
const send = <T>(path: string, body: unknown) => request<T>(path, body, true);
export const submissions = {
  list: (offset = 0) =>
    send<{ items: Submission[]; total: number }>("/video/mine", {
      offset,
      limit: 20,
    }),
  detail: (id: number) => send<Submission>("/video/submission", { id }),
  remove: (id: number) => send("/video/delete", { id }),
};
