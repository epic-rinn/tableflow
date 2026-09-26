import qrcode from "qrcode-generator";

// Renders a QR code as plain SVG rects from the encoder's module matrix (no
// HTML injection). Error correction M suits printed table cards.
export function QrCode({ value, label, size = 184 }: { value: string; label: string; size?: number }) {
  const qr = qrcode(0, "M");
  qr.addData(value, "Byte");
  qr.make();
  const n = qr.getModuleCount();
  const margin = 4;
  const cells: React.ReactNode[] = [];
  for (let r = 0; r < n; r++) {
    for (let c = 0; c < n; c++) {
      if (qr.isDark(r, c)) cells.push(<rect key={`${r}-${c}`} x={c + margin} y={r + margin} width={1} height={1} />);
    }
  }
  const total = n + margin * 2;
  return (
    <svg role="img" aria-label={label} viewBox={`0 0 ${total} ${total}`} width={size} height={size} shapeRendering="crispEdges" className="rounded-lg bg-white">
      <rect width={total} height={total} fill="#fff" />
      <g fill="#000">{cells}</g>
    </svg>
  );
}
