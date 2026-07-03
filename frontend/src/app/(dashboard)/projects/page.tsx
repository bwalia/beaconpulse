"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateProject, useProjects } from "@/lib/hooks";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, Field, Input, Select } from "@/components/ui";

const schema = z.object({
  name: z.string().min(1, "Name is required"),
  description: z.string().optional(),
  environment: z.enum(["production", "staging", "development"]),
});
type Values = z.infer<typeof schema>;

export default function ProjectsPage() {
  const { data, isLoading } = useProjects();
  const [showForm, setShowForm] = useState(false);

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Projects</h1>
          <p className="text-sm text-slate-500">Group your monitors by application or team.</p>
        </div>
        <Button onClick={() => setShowForm((v) => !v)}>{showForm ? "Close" : "New project"}</Button>
      </div>

      {showForm && <CreateProjectForm onDone={() => setShowForm(false)} />}

      {isLoading ? (
        <p className="text-slate-500">Loading…</p>
      ) : !data?.data.length ? (
        <Card>
          <p className="text-center text-slate-500">No projects yet. Create one to start adding monitors.</p>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {data.data.map((p) => (
            <Card key={p.id}>
              <div className="flex items-start justify-between">
                <h3 className="font-semibold">{p.name}</h3>
                <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs capitalize text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                  {p.environment}
                </span>
              </div>
              <p className="mt-1 text-sm text-slate-500">{p.description || "No description"}</p>
              <p className="mt-3 font-mono text-xs text-slate-400">{p.slug}</p>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

function CreateProjectForm({ onDone }: { onDone: () => void }) {
  const createProject = useCreateProject();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { environment: "production" } });

  const onSubmit = async (values: Values) => {
    setServerError(null);
    try {
      await createProject.mutateAsync(values);
      onDone();
    } catch (err) {
      setServerError(err instanceof ApiRequestError ? err.message : "Failed to create project");
    }
  };

  return (
    <Card>
      <form onSubmit={handleSubmit(onSubmit)} className="grid gap-4 sm:grid-cols-2">
        <Field label="Name" error={errors.name?.message}>
          <Input placeholder="Production Website" {...register("name")} />
        </Field>
        <Field label="Environment" error={errors.environment?.message}>
          <Select {...register("environment")}>
            <option value="production">Production</option>
            <option value="staging">Staging</option>
            <option value="development">Development</option>
          </Select>
        </Field>
        <div className="sm:col-span-2">
          <Field label="Description" error={errors.description?.message}>
            <Input placeholder="Optional" {...register("description")} />
          </Field>
        </div>
        {serverError && <p className="text-sm text-red-600 sm:col-span-2">{serverError}</p>}
        <div className="sm:col-span-2">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? "Creating…" : "Create project"}
          </Button>
        </div>
      </form>
    </Card>
  );
}
