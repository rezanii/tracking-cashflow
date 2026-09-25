import { apiClient, unwrap } from "@/lib/api-client";
import { authStorage } from "@/lib/auth-storage";
import type { LoginInput, RegisterInput } from "@/schemas";
import type { ApiEnvelope, LoginResult, User } from "@/types";

export const authService = {
  async login(input: LoginInput): Promise<LoginResult> {
    const { data } = await apiClient.post<ApiEnvelope<LoginResult>>("/auth/login", input);
    const result = unwrap(data);
    authStorage.save(result.access_token, result.user);
    return result;
  },

  async register(input: RegisterInput): Promise<User> {
    const { data } = await apiClient.post<ApiEnvelope<User>>("/auth/register", input);
    return unwrap(data);
  },

  async me(): Promise<User> {
    const { data } = await apiClient.get<ApiEnvelope<User>>("/auth/me");
    return unwrap(data);
  },

  // The token is stateless, so the local session is dropped even if the call fails.
  async logout(): Promise<void> {
    try {
      await apiClient.post<ApiEnvelope<null>>("/auth/logout");
    } finally {
      authStorage.clear();
    }
  },
};
