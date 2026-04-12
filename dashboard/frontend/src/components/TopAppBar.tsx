import { WindowMinimise, WindowToggleMaximise, Quit } from "../../wailsjs/runtime/runtime";

export function TopAppBar() {
  return (
    <header 
      // @ts-ignore
      style={{ '--wails-draggable': 'drag' }} 
      className="w-full h-8 flex items-center justify-between px-4 bg-white border-b border-zinc-100 shrink-0 select-none"
    >
      <div className="flex items-center gap-2">
        <span className="material-symbols-outlined text-blue-600 text-sm">terminal</span>
        <span className="font-mono font-bold text-zinc-900 text-sm">GhostApply</span>
      </div>
      
      {/* Exclude hover interactions from window-drag region using no-drag block */}
      <div 
        className="flex items-center gap-4 cursor-default text-zinc-400"
        // @ts-ignore
        style={{ '--wails-draggable': 'no-drag' }}
      >
        <button onClick={() => WindowMinimise()} className="hover:text-zinc-600 px-1 hover:bg-zinc-100 rounded transition-colors">—</button>
        <button onClick={() => WindowToggleMaximise()} className="hover:text-zinc-600 px-1 hover:bg-zinc-100 rounded transition-colors">□</button>
        <button onClick={() => Quit()} className="hover:text-red-500 px-1 hover:bg-red-50 rounded transition-colors">✕</button>
      </div>
    </header>
  );
}
