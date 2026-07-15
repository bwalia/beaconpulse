"use client";

import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useCreateProject, useProjectsPage } from "@/lib/hooks";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, EmptyState, Field, Input, PageHeader, Select, Skeleton } from "@/components/ui";
import { FolderIcon, PlusIcon, SearchIcon, XIcon } from "@/components/icons";
import { Pagination, SearchInput } from "@/components/table-controls";
import { useRevealVariants, useStaggerVariants } from "@/lib/motion";

const PROJECTS_PAGE_SIZE = 12;

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

// Left-border accent per environment, matching the Alerts list card style so the
// two pages read as one system.
const ENV_ACCENT: Record<string, string> = {
  production: "border-l-brand-600",
  staging: "border-l-amber-500",
  development: "border-l-slate-400 dark:border-l-slate-600",
};

export default function ProjectsPage() {
  const [page, setPage] = useState(0);
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [environment, setEnvironment] = useState("");
  const [showForm, setShowForm] = useState(false);
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.04);

  useEffect(() => {
    const t = setTimeout(() => setSearch(searchInput.trim()), 300);
    return () => clearTimeout(t);
  }, [searchInput]);
  useEffect(() => {
    setPage(0);
  }, [search, environment]);

  const { data, isLoading, isPlaceholderData } = useProjectsPage({
    page,
    pageSize: PROJECTS_PAGE_SIZE,
    search: search || undefined,
    environment: environment || undefined,
  });
  const rows = data?.data ?? [];
  const total = data?.pagination.total ?? 0;
  const filtering = search !== "" || environment !== "";

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

      {(total > 0 || filtering) && !isLoading && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <SearchInput
            value={searchInput}
            onChange={setSearchInput}
            placeholder="Search projects…"
            label="Search projects"
          />
          <select
            value={environment}
            onChange={(e) => setEnvironment(e.target.value)}
            aria-label="Filter by environment"
            className="rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 sm:w-48"
          >
            <option value="">All environments</option>
            <option value="production">Production</option>
            <option value="staging">Staging</option>
            <option value="development">Development</option>
          </select>
        </div>
      )}

      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-32 w-full rounded-xl" />
          ))}
        </div>
      ) : total === 0 ? (
        <EmptyState
          icon={filtering ? <SearchIcon className="h-5 w-5" /> : <FolderIcon className="h-5 w-5" />}
          title={filtering ? "No matching projects" : "No projects yet"}
          action={
            filtering ? (
              <Button
                variant="secondary"
                onClick={() => {
                  setSearchInput("");
                  setEnvironment("");
                }}
              >
                Clear filters
              </Button>
            ) : (
              <Button onClick={() => setShowForm(true)}>
                <PlusIcon className="h-4 w-4" />
                New project
              </Button>
            )
          }
        >
          {filtering
            ? "No projects match your search or filter."
            : "Projects group related monitors so alerts and dashboards stay organized by application or team."}
        </EmptyState>
      ) : (
        <>
          <motion.ul
            key={page}
            initial="hidden"
            animate="show"
            variants={stagger}
            className={`space-y-3 ${isPlaceholderData ? "opacity-60 transition-opacity" : "transition-opacity"}`}
          >
            {rows.map((p) => (
              <motion.li key={p.id} variants={reveal}>
                <Card
                  className={`border-l-4 transition-shadow hover:shadow-md motion-reduce:transition-none ${
                    ENV_ACCENT[p.environment] ?? ENV_ACCENT.development
                  }`}
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold text-slate-900 dark:text-white">{p.name}</span>
                        <span
                          className={`rounded-full px-2 py-0.5 text-xs font-semibold uppercase tracking-wide ${
                            ENV_STYLES[p.environment] ?? ENV_STYLES.development
                          }`}
                        >
                          {p.environment}
                        </span>
                      </div>
                      <p className="mt-1 truncate text-sm text-slate-600 dark:text-slate-300">
                        {p.description || "No description"}
                      </p>
                      <p className="truncate font-mono text-xs text-slate-500 dark:text-slate-400">{p.slug}</p>
                    </div>
                  </div>
                </Card>
              </motion.li>
            ))}
          </motion.ul>
          <Pagination
            page={page}
            pageSize={PROJECTS_PAGE_SIZE}
            total={total}
            unit="projects"
            busy={isPlaceholderData}
            onPageChange={setPage}
          />
        </>
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
