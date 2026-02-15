const EPORT_API_URL = process.env.EPORT_API_URL || "http://localhost:8080";
const EPORT_SERVICE_SECRET = process.env.EPORT_SERVICE_SECRET || "";

interface RequestOptions extends RequestInit {
  /** Override the base URL for this request */
  baseUrl?: string;
}

class EportClient {
  private baseUrl: string;
  private serviceSecret: string;

  constructor(baseUrl: string, serviceSecret: string) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.serviceSecret = serviceSecret;
  }

  private buildHeaders(
    additionalHeaders?: Record<string, string>
  ): HeadersInit {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      "X-Service-Secret": this.serviceSecret,
      ...additionalHeaders,
    };
    return headers;
  }

  async get(
    path: string,
    options?: {
      headers?: Record<string, string>;
      organizationId?: string;
    }
  ): Promise<Response> {
    const headers = this.buildHeaders(options?.headers);
    if (options?.organizationId) {
      (headers as Record<string, string>)["X-Organization-ID"] =
        options.organizationId;
    }

    return fetch(`${this.baseUrl}${path}`, {
      method: "GET",
      headers,
    });
  }

  async post(
    path: string,
    body?: unknown,
    options?: {
      headers?: Record<string, string>;
      organizationId?: string;
    }
  ): Promise<Response> {
    const headers = this.buildHeaders(options?.headers);
    if (options?.organizationId) {
      (headers as Record<string, string>)["X-Organization-ID"] =
        options.organizationId;
    }

    return fetch(`${this.baseUrl}${path}`, {
      method: "POST",
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  async put(
    path: string,
    body?: unknown,
    options?: {
      headers?: Record<string, string>;
      organizationId?: string;
    }
  ): Promise<Response> {
    const headers = this.buildHeaders(options?.headers);
    if (options?.organizationId) {
      (headers as Record<string, string>)["X-Organization-ID"] =
        options.organizationId;
    }

    return fetch(`${this.baseUrl}${path}`, {
      method: "PUT",
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  async patch(
    path: string,
    body?: unknown,
    options?: {
      headers?: Record<string, string>;
      organizationId?: string;
    }
  ): Promise<Response> {
    const headers = this.buildHeaders(options?.headers);
    if (options?.organizationId) {
      (headers as Record<string, string>)["X-Organization-ID"] =
        options.organizationId;
    }

    return fetch(`${this.baseUrl}${path}`, {
      method: "PATCH",
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  async delete(
    path: string,
    options?: {
      headers?: Record<string, string>;
      organizationId?: string;
    }
  ): Promise<Response> {
    const headers = this.buildHeaders(options?.headers);
    if (options?.organizationId) {
      (headers as Record<string, string>)["X-Organization-ID"] =
        options.organizationId;
    }

    return fetch(`${this.baseUrl}${path}`, {
      method: "DELETE",
      headers,
    });
  }

  /**
   * Forward a request to the e-port backend.
   * Used by the API proxy route to relay requests from the Shopify frontend.
   */
  async proxy(
    method: string,
    path: string,
    options: {
      body?: string;
      headers?: Record<string, string>;
      organizationId?: string;
    }
  ): Promise<Response> {
    const headers = this.buildHeaders(options.headers);
    if (options.organizationId) {
      (headers as Record<string, string>)["X-Organization-ID"] =
        options.organizationId;
    }

    return fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: options.body || undefined,
    });
  }
}

export const eportClient = new EportClient(EPORT_API_URL, EPORT_SERVICE_SECRET);
