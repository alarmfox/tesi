# Progetto tesi

## Esperimento

Creazione di un server TCP che accetta 2 tipi di richieste: una veloce e una lenta. Il server, in base alla richiesta, effettua due operazioni su una risorsa condivisa che ho pensato di simulare con un buffer. Ad esempio, la richiesta veloce legge l'ultimo valore inserito, mentre quella lenta effettua un'operazione più complicata come un inserimento ordinato (o qualcosa che possiamo simulare con una sleep).

L'obiettivo è quello di migliorare le prestazioni del server applicando un algoritmo a priorità invece di un FCFS.

Per generare il workload, avevo pensato di fare una piccola applicazione CLI che, a fronte di un numero di richieste totale da effettuare, permette di suddividere in percentuale il carico tra richieste lente e richieste veloci e di cambiare il tempo di interarrivo per tipologia di richiesta. Facendo variare questi parametri, possiamo raccogliere i valori delle performance (tempo di risposta, utilizzazione, numero di processi nel sistema) sia quando il server è in FCFS che con l'algoritmo a priorità.

Come tecnologia vorrei utilizzare GO (sia per la CLI che per il server), mentre come implementazione dell'algoritmo a priorità potrei utilizzare il Deficit Round Robin.

## Struttura elaborato

* Capitolo 1 Introduzione

    * 1.1 Algoritmi di short term scheduling: caratterizzazione qualitativa

    * 1.2 Teoria delle code
    
        * Processo di Poisson
            
        * 1.2.1 Modello a singola coda (M/M/1)

    * 1.3 Architettura client-server

        * 1.3.1 Connessioni TCP

* Capitolo 2 Problema: trashing

* Capitolo 3 Case study: slow client in un server TCP

    * 3.1 Descrizione del case study

    * 3.2 Caratterizzazione del trashing nel case study: parametri significativi

    * 3.3 Risoluzione del problema con un algoritmo di scheduling a priorità: Deficit Round Robin (DRR)

    * 3.4 Esperimento

        * 3.4.1 Metodologie utilizzate

        * 3.4.2 Creazione del workload

        * 3.4.3 Raccolta dati

    * 3.5 Risultati: comparazione prestazioni FCFS e DRR
