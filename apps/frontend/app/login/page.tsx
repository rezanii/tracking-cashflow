"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";

import { Button, Field, Input } from "@/components/ui";
import { useAuth } from "@/hooks/use-auth";
import { ApiError } from "@/lib/api-client";
import { loginSchema, registerSchema, type LoginInput, type RegisterInput } from "@/schemas";
import { authService } from "@/services/auth";

type Mode = "login" | "register";

export default function LoginPage() {
  const router = useRouter();
  const { user, loading, setUser } = useAuth();
  const [mode, setMode] = useState<Mode>("login");
  const [formError, setFormError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  // A visitor who already has a valid session should not see the form at all.
  useEffect(() => {
    if (!loading && user) {
      router.replace("/dashboard");
    }
  }, [loading, user, router]);

  const loginForm = useForm<LoginInput>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });

  const registerForm = useForm<RegisterInput>({
    resolver: zodResolver(registerSchema),
    defaultValues: { name: "", email: "", password: "", invite_code: "" },
  });

  // applyFieldErrors moves the API field errors onto the matching inputs so the user sees
  // them where they typed, not as one opaque banner.
  function applyFieldErrors(error: unknown, form: typeof loginForm | typeof registerForm) {
    if (error instanceof ApiError) {
      for (const [field, message] of Object.entries(error.fields)) {
        form.setError(field as never, { type: "server", message });
      }
      setFormError(Object.keys(error.fields).length > 0 ? null : error.message);
      return;
    }
    setFormError("Terjadi kesalahan tidak terduga");
  }

  async function handleLogin(values: LoginInput) {
    setFormError(null);
    setNotice(null);
    try {
      const result = await authService.login(values);
      setUser(result.user);
      router.replace("/dashboard");
    } catch (error) {
      applyFieldErrors(error, loginForm);
    }
  }

  async function handleRegister(values: RegisterInput) {
    setFormError(null);
    setNotice(null);
    try {
      await authService.register(values);
      // Registering does not sign the user in, so the email is carried over to the login form.
      setMode("login");
      loginForm.setValue("email", values.email);
      registerForm.reset();
      setNotice("Pendaftaran berhasil. Silakan masuk.");
    } catch (error) {
      applyFieldErrors(error, registerForm);
    }
  }

  const isLogin = mode === "login";
  const submitting = isLogin ? loginForm.formState.isSubmitting : registerForm.formState.isSubmitting;

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-100 px-4 py-10">
      <div className="w-full max-w-md">
        <div className="mb-6 text-center">
          <span className="inline-flex h-12 w-12 items-center justify-center rounded-xl bg-brand-600 text-lg font-bold text-white">
            Rp
          </span>
          <h1 className="mt-3 text-xl font-semibold text-slate-900">Tracking Cashflow</h1>
          <p className="mt-1 text-sm text-slate-500">Catat pemasukan dan pengeluaran Anda</p>
        </div>

        <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="mb-5 grid grid-cols-2 gap-1 rounded-lg bg-slate-100 p-1">
            {(["login", "register"] as const).map((value) => (
              <button
                key={value}
                type="button"
                onClick={() => {
                  setMode(value);
                  setFormError(null);
                }}
                className={
                  mode === value
                    ? "rounded-md bg-white px-3 py-2 text-sm font-semibold text-slate-900 shadow-sm"
                    : "rounded-md px-3 py-2 text-sm font-medium text-slate-600"
                }
              >
                {value === "login" ? "Masuk" : "Daftar"}
              </button>
            ))}
          </div>

          {notice && (
            <p className="mb-4 rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{notice}</p>
          )}
          {formError && (
            <p role="alert" className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
              {formError}
            </p>
          )}

          {isLogin ? (
            <form className="space-y-4" onSubmit={loginForm.handleSubmit(handleLogin)} noValidate>
              <Field label="Email" htmlFor="login-email" required error={loginForm.formState.errors.email?.message}>
                <Input
                  id="login-email"
                  type="email"
                  autoComplete="email"
                  placeholder="admin@example.com"
                  invalid={Boolean(loginForm.formState.errors.email)}
                  {...loginForm.register("email")}
                />
              </Field>

              <Field
                label="Password"
                htmlFor="login-password"
                required
                error={loginForm.formState.errors.password?.message}
              >
                <Input
                  id="login-password"
                  type="password"
                  autoComplete="current-password"
                  placeholder="••••••••"
                  invalid={Boolean(loginForm.formState.errors.password)}
                  {...loginForm.register("password")}
                />
              </Field>

              <Button type="submit" className="w-full" loading={submitting}>
                Masuk
              </Button>
            </form>
          ) : (
            <form className="space-y-4" onSubmit={registerForm.handleSubmit(handleRegister)} noValidate>
              <Field label="Nama" htmlFor="register-name" required error={registerForm.formState.errors.name?.message}>
                <Input
                  id="register-name"
                  autoComplete="name"
                  placeholder="John Doe"
                  invalid={Boolean(registerForm.formState.errors.name)}
                  {...registerForm.register("name")}
                />
              </Field>

              <Field label="Email" htmlFor="register-email" required error={registerForm.formState.errors.email?.message}>
                <Input
                  id="register-email"
                  type="email"
                  autoComplete="email"
                  placeholder="nama@example.com"
                  invalid={Boolean(registerForm.formState.errors.email)}
                  {...registerForm.register("email")}
                />
              </Field>

              <Field
                label="Password"
                htmlFor="register-password"
                required
                hint="Minimal 8 karakter, memuat huruf besar, huruf kecil dan angka"
                error={registerForm.formState.errors.password?.message}
              >
                <Input
                  id="register-password"
                  type="password"
                  autoComplete="new-password"
                  placeholder="••••••••"
                  invalid={Boolean(registerForm.formState.errors.password)}
                  {...registerForm.register("password")}
                />
              </Field>

              <Field
                label="Kode Undangan"
                htmlFor="register-invite"
                hint="Diperlukan bila pemilik aplikasi menutup pendaftaran. Kosongkan bila tidak punya."
                error={registerForm.formState.errors.invite_code?.message}
              >
                <Input
                  id="register-invite"
                  autoComplete="off"
                  placeholder="Kosongkan bila tidak diminta"
                  invalid={Boolean(registerForm.formState.errors.invite_code)}
                  {...registerForm.register("invite_code")}
                />
              </Field>

              <Button type="submit" className="w-full" loading={submitting}>
                Daftar
              </Button>
            </form>
          )}
        </div>

      </div>
    </div>
  );
}
