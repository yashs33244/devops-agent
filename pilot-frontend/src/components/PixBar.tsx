export function PixBar({ variant }: { variant?: "flip" | "cream" }) {
  return (
    <div
      className={`pix-bar${variant ? " " + variant : ""}`}
      aria-hidden="true"
    />
  );
}
