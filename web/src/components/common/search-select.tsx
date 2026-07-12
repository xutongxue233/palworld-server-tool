import { Check, ChevronsUpDown, Search } from "lucide-react";
import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";

export interface SearchOption {
  value: string;
  label: string;
  description?: string;
}

export function SearchSelect({
  value,
  options,
  placeholder,
  searchPlaceholder,
  emptyText = "No results",
  onValueChange,
  className,
}: {
  value?: string;
  options: SearchOption[];
  placeholder: string;
  searchPlaceholder?: string;
  emptyText?: string;
  onValueChange: (value: string) => void;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const selected = options.find((option) => option.value === value);
  const filtered = useMemo(() => {
    const normalized = search.trim().toLowerCase();
    const candidates = normalized
      ? options.filter((option) =>
          `${option.label} ${option.value} ${option.description ?? ""}`
            .toLowerCase()
            .includes(normalized),
        )
      : options;
    return candidates.slice(0, 80);
  }, [options, search]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className={cn("w-full justify-between font-normal", className)}
        >
          <span className="truncate">{selected?.label ?? placeholder}</span>
          <ChevronsUpDown className="shrink-0 text-muted-foreground" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-[min(360px,calc(100vw-2rem))] p-0"
      >
        <div className="relative border-b p-2">
          <Search className="pointer-events-none absolute left-5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={searchPlaceholder ?? placeholder}
            className="border-0 pl-9 shadow-none focus-visible:ring-0"
          />
        </div>
        <ScrollArea className="h-64">
          <div className="p-1">
            {filtered.length ? (
              filtered.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => {
                    onValueChange(option.value);
                    setOpen(false);
                    setSearch("");
                  }}
                  className="flex w-full items-center gap-3 rounded-sm px-2 py-2 text-left text-sm transition-colors hover:bg-accent focus-visible:bg-accent focus-visible:outline-none"
                >
                  <Check
                    className={cn(
                      "size-4 shrink-0",
                      value === option.value ? "opacity-100" : "opacity-0",
                    )}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate">{option.label}</span>
                    {option.description ? (
                      <span className="font-data block truncate text-[10px] text-muted-foreground">
                        {option.description}
                      </span>
                    ) : null}
                  </span>
                </button>
              ))
            ) : (
              <p className="px-3 py-8 text-center text-sm text-muted-foreground">
                {emptyText}
              </p>
            )}
          </div>
        </ScrollArea>
      </PopoverContent>
    </Popover>
  );
}
