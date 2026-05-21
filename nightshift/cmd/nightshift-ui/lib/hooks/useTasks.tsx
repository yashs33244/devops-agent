"use client";

import {
  useState,
  useReducer,
  useEffect,
  useRef,
  useCallback,
  useMemo,
  createContext,
  useContext,
  type ReactNode,
} from "react";
import { LoadingCenter, CenteredMessage } from "../ui";
import type { TaskResponse, StreamEvent } from "../api";
import * as api from "../api";
import { raw as rawEvent, titleFromPrompt } from "@/lib/events/parse";

export type TaskRun = {
  id: string;
  status: "pending" | "running" | "completed" | "error" | "interrupted";
  createdAt: number;
  startedAt?: string;
  completedAt?: string;
  iterations: number;
  promptVersion?: number;
  ranBy?: string;
  events: StreamEvent[];
};

export type Task = {
  id: string;
  agentId?: string | null;
  name: string;
  prompt: string;
  status: TaskResponse["status"];
  runs: TaskRun[];
};

export const isActiveRun = (r: { status: string }) => r.status === "running" || r.status === "pending";

export type TaskAction =
  | { type: "SET_TASKS"; tasks: Task[] }
  | { type: "ADD_TASK"; task: Task }
  | { type: "UPDATE_TASK"; taskId: string; name: string; prompt: string }
  | { type: "DELETE_TASK"; taskId: string }
  | { type: "START_RUN"; taskId: string; run: TaskRun }
  | { type: "STOP_RUN" | "RUN_COMPLETED" | "RUN_ERROR"; taskId: string; runId: string; completedAt: string }
  | { type: "BATCH_EVENTS"; events: { taskId: string; runId: string; event: StreamEvent }[]; completedAt: string }
  | { type: "LOAD_EVENTS"; taskId: string; runId: string; events: StreamEvent[] }
  | { type: "SYNC_RUN_STATUS"; taskId: string; runId: string; status: TaskRun["status"]; startedAt: string };

export function nameFromPrompt(prompt: string): string {
  return titleFromPrompt(prompt, "Untitled task");
}

function formatTs(ms: number | string): string {
  const d = new Date(ms);
  const month = d.toLocaleString("en-US", { month: "short" });
  const day = d.getDate();
  const time = d.toLocaleTimeString("en-US", { hour12: false });
  return `${month} ${day}, ${time}`;
}

function getTerminalStatus(event: StreamEvent): TaskRun["status"] | null {
  if (event.type.startsWith("result.")) {
    const raw = rawEvent(event);
    if (raw.is_error === true || event.type === "result.error") return "error";
    return "completed";
  }
  return null;
}

const RUN_TO_TASK_STATUS: Record<string, TaskResponse["status"]> = {
  completed: "completed", failed: "error", interrupted: "interrupted",
};

export function deriveRunMeta(events: StreamEvent[]): {
  startedAt?: string;
  completedAt?: string;
  iterations: number;
  status?: TaskRun["status"];
} {
  let startedAt: string | undefined;
  let completedAt: string | undefined;
  let iterations = 0;
  let status: TaskRun["status"] | undefined;

  for (const e of events) {
    if (!startedAt && e.timestamp) startedAt = formatTs(e.timestamp);
    if (e.type === "loop.iteration.start") iterations++;
    const terminal = getTerminalStatus(e);
    if (terminal) {
      status = terminal;
      if (e.timestamp) completedAt = formatTs(e.timestamp);
      const raw = (e.raw ?? e) as Record<string, unknown>;
      if (typeof raw.iterations === "number") iterations = raw.iterations;
    }
  }

  return { startedAt, completedAt, iterations, status };
}

export function toTask(job: TaskResponse): Task {
  const runs: TaskRun[] = (job.runs ?? []).map((r): TaskRun => ({
    id: r.id,
    status: r.status,
    createdAt: r.startedAt ? new Date(r.startedAt).getTime() : 0,
    startedAt: r.startedAt && r.status !== "pending" ? formatTs(new Date(r.startedAt).getTime()) : undefined,
    completedAt: r.completedAt ? formatTs(new Date(r.completedAt).getTime()) : undefined,
    iterations: r.iterations ?? 0,
    promptVersion: r.promptVersion ?? undefined,
    ranBy: r.ranBy ?? undefined,
    events: [],
  }));

  return {
    id: job.id,
    agentId: job.agentId,
    name: job.name || nameFromPrompt(job.prompt),
    prompt: job.prompt,
    status: job.status,
    runs,
  };
}

function updateRun(
  task: Task,
  runId: string,
  updater: (run: TaskRun) => TaskRun,
): Task {
  return {
    ...task,
    runs: task.runs.map((r) => (r.id === runId ? updater(r) : r)),
  };
}

export function taskReducer(tasks: Task[], action: TaskAction): Task[] {
  switch (action.type) {
    case "SET_TASKS":
      return action.tasks;

    case "ADD_TASK":
      return [...tasks, action.task];

    case "UPDATE_TASK":
      return tasks.map((t) =>
        t.id === action.taskId
          ? { ...t, name: action.name, prompt: action.prompt }
          : t,
      );

    case "DELETE_TASK":
      return tasks.filter((t) => t.id !== action.taskId);

    case "START_RUN":
      return tasks.map((t) =>
        t.id === action.taskId
          ? { ...t, status: "running", runs: [...t.runs, action.run] }
          : t,
      );

    case "STOP_RUN":
    case "RUN_COMPLETED":
    case "RUN_ERROR": {
      const statusMap = { STOP_RUN: "interrupted", RUN_COMPLETED: "completed", RUN_ERROR: "error" } as const;
      const runStatus = statusMap[action.type];
      const taskStatus = statusMap[action.type];
      return tasks.map((t) => {
        if (t.id !== action.taskId) return t;
        const updated = updateRun(t, action.runId, (r) => ({ ...r, status: runStatus, completedAt: action.completedAt }));
        const hasActive = updated.runs.some(isActiveRun);
        return { ...updated, status: hasActive ? "running" : taskStatus };
      });
    }

    case "BATCH_EVENTS": {
      // Group events by taskId for O(N+T) single-pass processing
      const byTask = new Map<string, { runId: string; event: StreamEvent }[]>();
      for (const ev of action.events) {
        let list = byTask.get(ev.taskId);
        if (!list) { list = []; byTask.set(ev.taskId, list); }
        list.push(ev);
      }

      return tasks.map((t) => {
        const taskEvents = byTask.get(t.id);
        if (!taskEvents) return t;

        let task = t;
        for (const { runId, event } of taskEvents) {
          task = updateRun(task, runId, (r) => ({
            ...r,
            events: [...r.events, event],
            iterations: event.type === "loop.iteration.start" ? r.iterations + 1 : r.iterations,
          }));

          if (event.type === "system") {
            task = { ...updateRun(task, runId, (r) => ({ ...r, status: "running" })), status: "running" };
          }
          const terminal = getTerminalStatus(event);
          if (terminal) {
            task = updateRun(task, runId, (r) => ({ ...r, status: terminal, completedAt: action.completedAt }));
            const hasActive = task.runs.some(isActiveRun);
            task = { ...task, status: hasActive ? "running" : (RUN_TO_TASK_STATUS[terminal] ?? task.status) };
          }
        }
        return task;
      });
    }

    case "LOAD_EVENTS": {
      const meta = deriveRunMeta(action.events);
      return tasks.map((t) => {
        if (t.id !== action.taskId) return t;
        return updateRun(t, action.runId, (r) => ({
          ...r,
          events: action.events,
          startedAt: r.startedAt || meta.startedAt,
          completedAt: r.completedAt || meta.completedAt,
          iterations: meta.iterations ?? r.iterations,
          status: meta.status ?? r.status,
        }));
      });
    }

    case "SYNC_RUN_STATUS": {
      const { taskId, runId, status, startedAt } = action;
      const targetTask = tasks.find((t) => t.id === taskId);
      const targetRun = targetTask?.runs.find((r) => r.id === runId);
      if (!targetRun || targetRun.status === status) return tasks;

      return tasks.map((t) => {
        if (t.id !== taskId) return t;
        const updated = updateRun(t, runId, (r) => {
          return { ...r, status, startedAt: status === "running" && !r.startedAt ? startedAt : r.startedAt };
        });
        const hasActive = updated.runs.some(isActiveRun);
        return { ...updated, status: hasActive ? "running" : (RUN_TO_TASK_STATUS[status] ?? t.status) };
      });
    }

    default:
      return tasks;
  }
}

function now() {
  return formatTs(Date.now());
}

/**
 * Merge server task responses with existing client state.
 * Preserves client-side SSE events/status for known runs, appends new runs from DB.
 */
export function mergeTasksFromServer(
  clientTasks: Task[],
  serverResponses: TaskResponse[],
): Task[] {
  const existingRuns = new Map(clientTasks.map((t) => [t.id, t.runs]));
  return serverResponses.map((j) => {
    const converted = toTask(j);
    const clientRuns = existingRuns.get(j.id);
    if (!clientRuns) return converted;

    const knownIds = new Set(clientRuns.map((r) => r.id));
    const newRuns = converted.runs.filter((r) => !knownIds.has(r.id));
    return { ...converted, runs: [...clientRuns, ...newRuns] };
  });
}

function useTasks(userName: string) {
  const [tasks, dispatch] = useReducer(taskReducer, []);
  const [loaded, setLoaded] = useState(false);
  const loadedRuns = useRef<Set<string>>(new Set());
  const tasksRef = useRef(tasks);
  tasksRef.current = tasks;
  const userNameRef = useRef(userName);
  userNameRef.current = userName;

  const fetchTasks = useCallback(async () => {
    try {
      const jobs = await api.fetchTasks();
      const newTasks = mergeTasksFromServer(tasksRef.current, jobs);
      dispatch({ type: "SET_TASKS", tasks: newTasks });
    } catch (e) {
      console.error("[useTasks] fetchTasks failed:", e);
    } finally {
      setLoaded(true);
    }
  }, []);

  useEffect(() => {
    fetchTasks();
  }, [fetchTasks]);

  useEffect(() => {
    const id = setInterval(() => {
      if (document.hidden) return;
      fetchTasks();
    }, 60_000);

    const onVisible = () => {
      if (!document.hidden) fetchTasks();
    };
    document.addEventListener("visibilitychange", onVisible);

    return () => {
      clearInterval(id);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [fetchTasks]);

  // Run status polling removed — demo UI only

  const pendingEvents = useRef<{ taskId: string; runId: string; event: StreamEvent }[]>([]);
  const rafId = useRef<number>(0);
  const sseActiveRuns = useRef<Set<string>>(new Set());

  const flushEvents = useCallback(() => {
    rafId.current = 0;
    const batch = pendingEvents.current.splice(0);
    if (batch.length === 0) return;
    dispatch({ type: "BATCH_EVENTS", events: batch, completedAt: now() });
  }, []);

  useEffect(() => {
    return () => {
      if (rafId.current) cancelAnimationFrame(rafId.current);
    };
  }, []);

  // Track last seen event index per run for cursor-based deduplication
  const runCursors = useRef<Map<string, number>>(new Map());

  const subscribeToRun = useCallback((taskId: string, runId: string, onError?: () => void) => {
    const afterIndex = runCursors.current.get(runId) ?? -1;
    sseActiveRuns.current.add(runId);
    const es = api.subscribeToEvents((event) => {
      // Server sends an error event when SSE is unavailable (e.g., run completed)
      if (event.type === "error") {
        console.warn("[useTasks] SSE unavailable for run", runId, rawEvent(event).error);
        sseActiveRuns.current.delete(runId);
        es.close();
        onError?.();
        return;
      }
      if (typeof event.index === "number") {
        const prev = runCursors.current.get(runId) ?? -1;
        if (event.index > prev) runCursors.current.set(runId, event.index);
      }
      pendingEvents.current.push({ taskId, runId, event });
      if (!rafId.current) {
        rafId.current = requestAnimationFrame(flushEvents);
      }
    }, runId, afterIndex);
    es.onerror = () => {
      console.warn("[useTasks] SSE connection error for run", runId);
      sseActiveRuns.current.delete(runId);
      es.close();
      onError?.();
    };
    return () => {
      sseActiveRuns.current.delete(runId);
      es.close();
    };
  }, [flushEvents]);

  const createTask = useCallback(
    async (data: { name: string; prompt: string }) => {
      const job = await api.createTask(data);
      const task = { ...toTask(job), name: data.name };
      dispatch({ type: "ADD_TASK", task });
      return task;
    },
    [],
  );

  const updateTask = useCallback(
    async (taskId: string, data: { name: string; prompt: string }) => {
      await api.updateTask(taskId, data);
      dispatch({
        type: "UPDATE_TASK",
        taskId,
        name: data.name,
        prompt: data.prompt,
      });
    },
    [],
  );

  // Delete task — optimistic: update UI first, then call API
  const deleteTask = useCallback(async (taskId: string) => {
    const task = tasksRef.current.find((t) => t.id === taskId);
    dispatch({ type: "DELETE_TASK", taskId });
    if (task) {
      for (const r of task.runs) {
        runCursors.current.delete(r.id);
        loadedRuns.current.delete(r.id);
      }
    }
    api.deleteTask(taskId).catch((e) => {
      console.error("[useTasks] deleteTask failed:", e);
    });
  }, []);

  const runningRef = useRef<Set<string>>(new Set());

  const runTask = useCallback(async (taskId: string, promptOverride?: string) => {
    const task = tasksRef.current.find((t) => t.id === taskId);
    if (!task || task.runs.some(isActiveRun) || runningRef.current.has(taskId)) return;
    runningRef.current.add(taskId);
    try {
      const res = await api.startRun(taskId, promptOverride ?? task.prompt, task.agentId ?? undefined);
      const runStatus: TaskRun["status"] = res.status === "pending" ? "pending" : "running";
      const newRun: TaskRun = {
        id: res.id,
        status: runStatus,
        createdAt: Date.now(),
        startedAt: runStatus === "pending" ? undefined : now(),
        iterations: 0,
        ranBy: userNameRef.current,
        events: [],
      };
      dispatch({ type: "START_RUN", taskId, run: newRun });
    } finally {
      runningRef.current.delete(taskId);
    }
  }, []);

  const stopTask = useCallback(async (taskId: string) => {
    const task = tasksRef.current.find((t) => t.id === taskId);
    if (!task) return;
    const activeRun = task.runs.find(isActiveRun);
    if (!activeRun) return;
    await api.interruptRun(activeRun.id);
    dispatch({
      type: "STOP_RUN",
      taskId,
      runId: activeRun.id,
      completedAt: now(),
    });
  }, []);

  const loadStreamEvents = useCallback(async (taskId: string, runId: string) => {
    if (loadedRuns.current.has(runId)) return;
    loadedRuns.current.add(runId);
    try {
      const events = await api.fetchRunEvents(runId);
      dispatch({ type: "LOAD_EVENTS", taskId, runId, events });
      // Set cursor to max index from loaded history
      const maxIndex = events.reduce((max, e) => {
        const idx = typeof e.index === "number" ? e.index : -1;
        return idx > max ? idx : max;
      }, -1);
      if (maxIndex >= 0) runCursors.current.set(runId, maxIndex);
    } catch {
      loadedRuns.current.delete(runId);
    }
  }, []);

  const state = useMemo(() => ({ tasks, loaded }), [tasks, loaded]);
  const actions = useMemo(() => ({
    createTask,
    updateTask,
    deleteTask,
    runTask,
    stopTask,
    loadStreamEvents,
    subscribeToRun,
    refetch: fetchTasks,
  }), [
    createTask,
    updateTask,
    deleteTask,
    runTask,
    stopTask,
    loadStreamEvents,
    subscribeToRun,
    fetchTasks,
  ]);

  return { state, actions };
}

type TasksState = { tasks: Task[]; loaded: boolean };
type TasksActions = ReturnType<typeof useTasks>["actions"];
type TasksContextValue = TasksState & TasksActions;

const TasksStateContext = createContext<TasksState | null>(null);
const TasksActionsContext = createContext<TasksActions | null>(null);

export function TasksProvider({ userName, children }: { userName: string; children: ReactNode }) {
  const { state, actions } = useTasks(userName);
  return (
    <TasksActionsContext.Provider value={actions}>
      <TasksStateContext.Provider value={state}>
        {children}
      </TasksStateContext.Provider>
    </TasksActionsContext.Provider>
  );
}

/** Subscribe to tasks state (tasks, loaded). Re-renders when tasks change. */
export function useTasksState(): TasksState {
  const ctx = useContext(TasksStateContext);
  if (!ctx) throw new Error("useTasksState must be used within TasksProvider");
  return ctx;
}

/** Subscribe to tasks actions only. Stable refs — never causes re-renders. */
export function useTasksActions(): TasksActions {
  const ctx = useContext(TasksActionsContext);
  if (!ctx) throw new Error("useTasksActions must be used within TasksProvider");
  return ctx;
}

/** Compat wrapper: subscribe to both state + actions. */
export function useTasksContext(): TasksContextValue {
  return { ...useTasksState(), ...useTasksActions() };
}

export function TaskGuard({ taskId, children }: { taskId: string; children: ReactNode }) {
  const { tasks, loaded } = useTasksState();
  const task = tasks.find((t) => t.id === taskId);
  if (!loaded) return <LoadingCenter />;
  if (!task) return <CenteredMessage>Task not found</CenteredMessage>;
  return <>{children}</>;
}
