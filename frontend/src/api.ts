const apiBase =
  import.meta.env.VITE_API_BASE_URL ??
  "/api/v1/physec/accesscontrol/psap";

type APIError = {
  error?: string;
};

export async function request<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  const body = await response.text();
  let result: unknown;
  if (body !== "") {
    try {
      result = JSON.parse(body);
    } catch {
      throw new Error(`The API returned an unreadable response (${response.status}).`);
    }
  }

  if (!response.ok) {
    const message =
      typeof result === "object" &&
      result !== null &&
      "error" in result &&
      typeof (result as APIError).error === "string"
        ? (result as APIError).error
        : `The API request failed (${response.status}).`;
    throw new Error(message);
  }
  if (result === undefined) {
    throw new Error("The API returned an empty response.");
  }
  return result as T;
}

export function get<T>(path: string): Promise<T> {
  return request<T>(path);
}

export function post<T>(path: string, value: unknown): Promise<T> {
  return request<T>(path, {
    method: "POST",
    body: JSON.stringify(value),
  });
}
