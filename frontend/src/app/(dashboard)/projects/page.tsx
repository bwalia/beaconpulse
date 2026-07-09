"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateProject, useProjects } from "@/lib/hooks";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, EmptyState, Field, Input, PageHeader, Select, Skeleton } from "@/components/ui";
import { FolderIcon, PlusIcon, XIcon } from "@/components/icons";

const schema = z.object({
  name: z.string().min(1, "Name is required"),
  description: z.string().optional(),
  environment: z.enum(["production", "staging", "development"]),
});
type Values = z.infer<typeof schema>;

// Tints chosen so the label text clears 4.5:1 against its own background.
const ENV_STYLES: Record<string, string> = {
  production: "bg-brand-100 text-brand-800 dark:bg-brand-900/40 dark:text-brand-200",
  staging: "bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-200",
  development: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300",
};

export default function ProjectsPage() {
  const { data, isLoading } = useProjects();
  const [showForm, setShowForm] = useState(false);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Projects"
        subtitle="Group your monitors by application or team."
        actions={
          <Button onClick={() => setShowForm((v) => !v)}>
            {showForm ? <XIcon className="h-4 w-4" /> : <PlusIcon className="h-4 w-4" />}
            {showForm ? "Close" : "New project"}
          </Button>
        }
      />

      {showForm && <CreateProjectForm onDone={() => setShowForm(false)} />}

      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-32 w-full rounded-xl" />
          ))}
        </div>
      ) : !data?.data.length ? (
        <EmptyState
          icon={<FolderIcon className="h-5 w-5" />}
          title="No projects yet"
          action={
            <Button onClick={() => setShowForm(true)}>
              <PlusIcon className="h-4 w-4" />
              New project
            </Button>
          }
        >
          Projects group related monitors so alerts and dashboards stay organized by application or team.
        </EmptyState>
      ) : (
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {data.data.map((p) => (
            <li key={p.id}>
              <Card className="h-full transition-shadow hover:shadow-md motion-reduce:transition-none">
                <div className="flex items-start justify-between gap-3">
                  <h3 className="truncate font-semibold">{p.name}</h3>
                  <span
                    className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-semibold uppercase tracking-wide ${
                      ENV_STYLES[p.environment] ?? ENV_STYLES.development
                    }`}
                  >
                    {p.environment}
                  </span>
                </div>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{p.description || "No description"}</p>
                <p className="mt-3 truncate font-mono text-xs text-slate-500 dark:text-slate-400">{p.slug}</p>
              </Card>
            </li>
          ))}
        </ul>
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
          <Field label="Description" hint="Optional — what this project covers." error={errors.description?.message}>
            <Input placeholder="Marketing site and its APIs" {...register("description")} />
          </Field>
        </div>
        {serverError && (
          <p role="alert" className="text-sm font-medium text-red-700 sm:col-span-2 dark:text-red-400">
            {serverError}
          </p>
        )}
        <div className="flex gap-2 sm:col-span-2">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? "Creating…" : "Create project"}
          </Button>
          <Button type="button" variant="secondary" onClick={onDone}>
            Cancel
          </Button>
        </div>
      </form>
    </Card>
  );
}
