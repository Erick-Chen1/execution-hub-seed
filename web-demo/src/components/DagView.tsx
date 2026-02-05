import React, { useMemo } from 'react';
import ReactFlow, { Background, Controls, Edge, Node } from 'reactflow';
import 'reactflow/dist/style.css';

interface DagViewProps {
  steps: { id: string; name: string; status?: string }[];
  edges: { from: string; to: string }[];
}

function buildLayout(steps: DagViewProps['steps'], edges: DagViewProps['edges']) {
  const deps = new Map<string, string[]>();
  steps.forEach((s) => deps.set(s.id, []));
  edges.forEach((e) => {
    const list = deps.get(e.to) || [];
    list.push(e.from);
    deps.set(e.to, list);
  });

  const level = new Map<string, number>();
  const visit = (id: string): number => {
    if (level.has(id)) return level.get(id)!;
    const parents = deps.get(id) || [];
    const l = parents.length ? Math.max(...parents.map(visit)) + 1 : 0;
    level.set(id, l);
    return l;
  };
  steps.forEach((s) => visit(s.id));

  const levels = new Map<number, string[]>();
  steps.forEach((s) => {
    const l = level.get(s.id) || 0;
    const list = levels.get(l) || [];
    list.push(s.id);
    levels.set(l, list);
  });

  const nodes: Node[] = steps.map((s) => {
    const l = level.get(s.id) || 0;
    const idx = (levels.get(l) || []).indexOf(s.id);
    return {
      id: s.id,
      position: { x: l * 260, y: idx * 120 },
      data: {
        label: (
          <div style={{ fontSize: 12 }}>
            <div style={{ fontWeight: 600 }}>{s.name}</div>
            {s.status && <div style={{ color: '#64748b' }}>{s.status}</div>}
          </div>
        )
      },
      style: {
        border: '1px solid #d9dde7',
        borderRadius: 12,
        padding: 8,
        background: '#ffffff',
        minWidth: 160
      }
    };
  });

  const flowEdges: Edge[] = edges.map((e) => ({
    id: `${e.from}-${e.to}`,
    source: e.from,
    target: e.to
  }));

  return { nodes, edges: flowEdges };
}

export default function DagView({ steps, edges }: DagViewProps) {
  const layout = useMemo(() => buildLayout(steps, edges), [steps, edges]);
  return (
    <div style={{ height: 360, border: '1px solid var(--border)', borderRadius: 14 }}>
      <ReactFlow nodes={layout.nodes} edges={layout.edges} fitView>
        <Background gap={18} size={1} color="#e2e8f0" />
        <Controls />
      </ReactFlow>
    </div>
  );
}
