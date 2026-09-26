type Props = {
  id: string;
  label: string;
  error?: string;
} & React.InputHTMLAttributes<HTMLInputElement>;

export function Field({ id, label, error, ...input }: Props) {
  return (
    <p>
      <label htmlFor={id}>{label}</label>
      <input id={id} name={id} aria-invalid={!!error} aria-describedby={error ? `${id}-error` : undefined} {...input} />
      {error && <span id={`${id}-error`}> {error}</span>}
    </p>
  );
}
