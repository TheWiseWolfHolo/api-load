import http from "@/utils/http";

const AUTH_KEY = "authKey";

const login = async (authKey: string): Promise<boolean> => {
  try {
    await http.post("/auth/login", { auth_key: authKey });
    localStorage.setItem(AUTH_KEY, authKey);
    return true;
  } catch (error) {
    console.error("Login failed:", error);
    return false;
  }
};

const logout = (): void => {
  localStorage.removeItem(AUTH_KEY);
};

const getAuthKey = (): string | null => {
  return localStorage.getItem(AUTH_KEY);
};

const isLoggedIn = (): boolean => {
  return !!localStorage.getItem(AUTH_KEY);
};

export const authService = {
  login,
  logout,
  getAuthKey,
  isLoggedIn,
};
