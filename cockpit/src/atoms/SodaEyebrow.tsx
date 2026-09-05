const symbol = new URL("../../../assets/branding/source/soda-symbol.svg", import.meta.url).href;

export function SodaEyebrow() {
  return (
    <p className="soda-eyebrow">
      <img src={symbol} width={32} height={32} alt="" />
      Soda OS
    </p>
  );
}
